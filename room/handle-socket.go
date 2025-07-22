package room

import (
	"errors"
	"github.com/gobwas/ws"
	"net/http"
	"time"
)

type GetPlayerIdFromRequester[PlayerId comparable] interface {
	GetPlayerIdFromRequest(w http.ResponseWriter, r *http.Request) PlayerId
}
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

func (room *Room[RoomId, PlayerId]) HandleSocketWithPlayer(playerId PlayerId, onError ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var zero PlayerId
		if playerId == zero {
			onError(w, r, errors.New("playerId is nil"))
			return
		}

		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			onError(w, r, err)
			return
		}
		room.Slogger.Info("new socket connection", "player", playerId)

		ss := NewSocketSession[PlayerId](conn, playerId, room.messages)

		room.mu.Lock()
		room.players[playerId] = ss
		room.mu.Unlock()

		go func() {
			<-time.After(time.Millisecond * 1)
			room.OnConnect(playerId)
		}()
	}
}

func (room *Room[RoomId, PlayerId]) HandleSocket(playerStore GetPlayerIdFromRequester[PlayerId], onError ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerId := playerStore.GetPlayerIdFromRequest(w, r)
		room.HandleSocketWithPlayer(playerId, onError)(w, r)
	}
}
