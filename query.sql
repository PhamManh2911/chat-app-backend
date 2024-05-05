-- name: GetUser :one
SELECT * FROM "User" WHERE email=$1 LIMIT 1;

-- name: GetChat :one
SELECT * FROM "Chat" WHERE id=$1 LIMIT 1;

-- name: CreateMessage :one
INSERT INTO "Message" ("value", "updatedAt", "chatId", "userToChatId") VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserToChat :one
SELECT "id" FROM "UserToChat" WHERE "userId"=$1 AND "chatId"=$2 LIMIT 1;
