package room

import (
	"github.com/gobwas/ws"
	"net/http"
)

type GetPlayerIdFromRequester[PlayerId comparable] interface {
	GetPlayerIdFromRequest(w http.ResponseWriter, r *http.Request) PlayerId
}
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

func (room *Room[RoomId, PlayerId]) HandleSocketWithPlayer(playerId PlayerId, onError ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		go room.OnConnect(playerId)
	}
}

func (room *Room[RoomId, PlayerId]) HandleSocket(playerStore GetPlayerIdFromRequester[PlayerId], onError ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerId := playerStore.GetPlayerIdFromRequest(w, r)
		//if playerId == nil {
		//	onError(w, r, fmt.Errorf("player not found"))
		//	return
		//}
		room.HandleSocketWithPlayer(playerId, onError)(w, r)
	}
}
