package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"main/database"
	"main/server"
	"time"

	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/net/websocket"
)

type Payload struct {
	Id int `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Value string `json:"value"`
	UserSentId int `json:"userSentId"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := database.New(pool)

	connIdentity := server.NewConnIdentity(queries, ctx)

	http.Handle("/ws", websocket.Handler(func (ws *websocket.Conn) {
		identity, err := connIdentity.GetOrCreateConnIdentity(ws)

		if err != nil {
			ws.Close()
			return
		}
		defer connIdentity.RemoveConn(ws)
		fmt.Println("userToChat", identity.UserToChatId, "connected")

		chann := make(chan string)

		go connIdentity.ListeningForMessage(ws, chann)

		for i := range chann {
			fmt.Println("userToChat", identity.UserToChatId, "says: ", i)
			message, err := queries.CreateMessage(ctx, database.CreateMessageParams{
				Value: i,
				UpdatedAt: pgtype.Timestamp{Time: time.Now().Round(time.Millisecond), Valid: true},
				ChatId: int32(identity.ChatId),
				UserToChatId: int32(identity.UserToChatId),
			})

			if err != nil {
				fmt.Println(err)
				return
			}

			payload := Payload{
				Id: int(message.ID),
				CreatedAt: message.CreatedAt.Time,
				UpdatedAt: message.UpdatedAt.Time,
				Value: message.Value,
				UserSentId: identity.UserToChatId,
			}
			b, err := json.Marshal(payload)

			if err != nil {
				fmt.Println(err)
				return
			}
			connIdentity.BroadcastToRoom(identity.ChatId, b)
		}
	}))

	fmt.Println("Server listening on port 8000")
	http.ListenAndServe(":8000", nil)
}