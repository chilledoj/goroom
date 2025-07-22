package chat

import "time"

type UserId = int

type User struct {
	Id   UserId `json:"id"`
	Name string `json:"username"`
}

type MessageType = string

const (
	MT_Message    MessageType = "message"
	MT_UserJoined MessageType = "user.joined"
	MT_UserLeft   MessageType = "user.left"
)

type ChatMessage struct {
	Sender  User      `json:"sender"`
	Message string    `json:"message"`
	Tsp     time.Time `json:"tsp"`
}

type Message struct {
	MessageType `json:"messageType"`
	ChatMessage `json:"chatMessage"`
}
