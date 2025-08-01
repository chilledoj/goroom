package goroom

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
		if !room.CanJoin(playerId) {
			onError(w, r, errors.New("player cannot join room"))
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
			room.opts.OnConnect(playerId)
		}()
	}
}

func (room *Room[RoomId, PlayerId]) HandleSocket(playerStore GetPlayerIdFromRequester[PlayerId], onError ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerId := playerStore.GetPlayerIdFromRequest(w, r)
		room.HandleSocketWithPlayer(playerId, onError)(w, r)
	}
}

func (room *Room[RoomId, PlayerId]) CanJoin(playerId PlayerId) bool {
	if room.Status == Inactive {
		return false
	}

	// Open OR Locked
	room.mu.RLock()
	defer room.mu.RUnlock()

	p, ok := room.players[playerId]
	if ok && p != nil {
		// Player is already connected. Only allow one connection.
		return false
	}
	if !ok && room.Status == Locked {
		// Locked Room and player was not previously connected when locked
		return false
	}

	/*
		We can safely assume now that `p==nil` and that either:
		+ `(!ok & room.Status == Open)`: the player is not connected, and the room is open to new connections
		+ `(ok)`: the player was previously connected.
	*/
	return true
}
