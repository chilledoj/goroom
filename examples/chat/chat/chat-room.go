package chat

import (
	"context"
	"encoding/json"
	"github.com/chilledoj/goroom/room"
	"log/slog"
	"sync"
)

type ChatRoom struct {
	*room.Room[string, UserId]

	mu   sync.Mutex
	msgs []ChatMessage

	users UserStore

	Slogger *slog.Logger
}

func NewChatRoom(ctx context.Context) *ChatRoom {
	cr := &ChatRoom{
		users:   NewUserStore(),
		msgs:    make([]ChatMessage, 0),
		Slogger: slog.Default(),
	}

	cr.Room = room.NewRoom[string, UserId](ctx, "room", room.Options[UserId]{
		OnConnect:    cr.OnConnect,
		OnDisconnect: cr.OnDisconnect,
		OnMessage:    cr.OnMessage,
		Slogger:      cr.Slogger,
	})

	return cr
}

func (cr *ChatRoom) Start() {
	cr.Room.Start()
}
func (cr *ChatRoom) Stop() {
	cr.Room.Stop()
}

func (cr *ChatRoom) NewUser(name string) *User {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	user := cr.users.AddUser(name)
	return user
}

func (cr *ChatRoom) OnConnect(playerId UserId) {
	cr.Slogger.Info("player connected", "playerId", playerId)

	user := cr.users.GetUser(playerId)

	usrInfoMsgObject := Message{
		MessageType: MT_UserJoined,
		ChatMessage: ChatMessage{
			Sender: User{
				Id:   user.Id,
				Name: user.Name,
			},
			Message: "user " + user.Name + " joined the chat",
		},
	}
	userInfoMsg, _ := json.Marshal(usrInfoMsgObject)

	cr.Room.SendMessageToPlayer(playerId, userInfoMsg)

	msgObject := Message{
		MessageType: MT_UserJoined,
		ChatMessage: ChatMessage{
			Sender: User{
				Id:   0,
				Name: "system",
			},
			Message: "user " + user.Name + " joined the chat",
		},
	}
	msg, _ := json.Marshal(msgObject)

	cr.Room.SendMessageToAllPlayers(msg)
}

func (cr *ChatRoom) OnDisconnect(playerId UserId) {
	cr.Slogger.Info("player disconnected", "playerId", playerId)
	cr.mu.Lock()
	defer cr.mu.Unlock()
	user := cr.users.GetUser(playerId)

	msgObject := Message{
		MessageType: MT_UserLeft,
		ChatMessage: ChatMessage{
			Sender: User{
				Id:   0,
				Name: "system",
			},
			Message: "user " + user.Name + " left the chat",
		},
	}
	msg, _ := json.Marshal(msgObject)
	cr.SendMessageToAllPlayers(msg)
	cr.users.RemoveUser(playerId)
}

func (cr *ChatRoom) OnMessage(playerId UserId, message []byte) {
	cr.Slogger.Info("player sent message", "playerId", playerId, "message", string(message))
	cr.mu.Lock()
	defer cr.mu.Unlock()

	var cm Message
	if err := json.Unmarshal(message, &cm); err != nil {
		cr.Slogger.Error("error unmarshalling message", "err", err)
	}

	cr.msgs = append(cr.msgs, cm.ChatMessage)
	cr.SendMessageToAllPlayers(message)
}
