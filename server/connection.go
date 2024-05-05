package server

import (
	"context"
	"fmt"
	"io"
	"main/api"
	"main/database"
	"net/http"
	"os"
	"strconv"
	"sync"

	"golang.org/x/net/websocket"
)

type Claim struct {
	Email string `json:"email"`
}

type Identity struct {
	UserToChatId int
	ChatId int
}

// map websocker connection with user-chat identity
type ConnIdentity struct {
	mu sync.Mutex
	Conns map[*websocket.Conn]*Identity
	queries *database.Queries
	ctx context.Context
}

func NewConnIdentity(queries *database.Queries, ctx context.Context) *ConnIdentity {
	return &ConnIdentity{
		queries: queries,
		ctx: ctx,
		Conns: make(map[*websocket.Conn]*Identity),
	}
}

func (connIdentity *ConnIdentity) CreateConnIdentity(conn *websocket.Conn, chatId int, userToChatId int) *Identity {
	identity := &Identity{UserToChatId: userToChatId, ChatId: chatId}

	connIdentity.mu.Lock()
	defer connIdentity.mu.Unlock()
	connIdentity.Conns[conn] = identity

	return identity
}

func (connIdentity *ConnIdentity) RemoveConn(conn *websocket.Conn) {
	connIdentity.mu.Lock()
	defer connIdentity.mu.Unlock()

	delete(connIdentity.Conns, conn)
}

func (connIdentity *ConnIdentity) GetOrCreateConnIdentity(conn *websocket.Conn) (*Identity, error) {
	var identity *Identity
	identity, ok := connIdentity.Conns[conn]

	if ok {
		fmt.Println("connection existed")
		return identity, nil
	}

	queryString := conn.Config().Location.Query()
	chatId, err := strconv.Atoi(queryString["chat_id"][0])

	if err != nil {
		conn.Close()
		return nil, err
	}
	
	if _, err := connIdentity.queries.GetChat(connIdentity.ctx, int32(chatId)); err != nil {
		fmt.Println("Chat Id", chatId, "does not exist")
		fmt.Println(err)
		conn.Close()
		return nil, err
	}
	
	accessToken := queryString["access_token"][0]
	userInfoEndpoint := fmt.Sprintf("https://%s/userinfo", os.Getenv("AUTH0_DOMAIN"))
	req, err := http.NewRequest("GET", userInfoEndpoint, nil)
	
	if err != nil {
		conn.Close()
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	
	data := &Claim{}
	
	if err := api.GetResponse(req, data); err != nil {
		fmt.Println("Can not get identity from auth0")
		conn.Close()
		return nil, err
	}

	user, err := connIdentity.queries.GetUser(connIdentity.ctx, data.Email)

	if err != nil {
		conn.Close()
		return nil, err
	}

	userToChatParams := database.GetUserToChatParams{UserId: user.ID, ChatId: int32(chatId)}

	userToChat, err := connIdentity.queries.GetUserToChat(connIdentity.ctx, userToChatParams)

	if err != nil {
		conn.Close()
		return nil, err
	}

	return connIdentity.CreateConnIdentity(conn, chatId, int(userToChat)), nil
}

func (connIdentity *ConnIdentity) ListeningForMessage(ws *websocket.Conn, c chan string) {
	buffer := make([]byte, 1024)

	for {
		n, err := ws.Read(buffer)

		if err != nil {
			if err == io.EOF {
				fmt.Println("userToChat", connIdentity.Conns[ws].UserToChatId, "disconnected")
				break
			}

			continue
		}

		c <- string(buffer[:n])
	}

	close(c)
}

func (connIdentity *ConnIdentity) BroadcastToRoom(roomId int, payload []byte) {
	for conn, identity := range connIdentity.Conns {
		if identity.ChatId != roomId {
			continue
		}
		go func(ws *websocket.Conn, pl []byte) {
			if _, err := ws.Write(pl); err != nil {
				fmt.Println("error: ", err)
			}
		}(conn, payload)
	}
}