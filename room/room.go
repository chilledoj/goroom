package room

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Room[RoomId comparable, PlayerId comparable] struct {
	ID RoomId
	Options[PlayerId]

	mu            sync.RWMutex
	players       map[PlayerId]*SocketSession[PlayerId]
	lastSeen      map[PlayerId]time.Time
	cleanupPeriod time.Duration

	// MessageProcessing
	messages chan SocketMessage[PlayerId]

	// Concurrency
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Logging
	Slogger *slog.Logger
}

type Options[PlayerId comparable] struct {
	OnConnect    func(player PlayerId)
	OnDisconnect func(player PlayerId)
	OnMessage    func(player PlayerId, message []byte)

	CleanupPeriod time.Duration

	Slogger *slog.Logger
}

func NewRoom[RoomId comparable, PlayerId comparable](parentCtx context.Context, id RoomId, options Options[PlayerId]) *Room[RoomId, PlayerId] {
	ctx, cancel := context.WithCancel(parentCtx)
	room := &Room[RoomId, PlayerId]{
		ID:       id,
		Options:  options,
		players:  make(map[PlayerId]*SocketSession[PlayerId]),
		messages: make(chan SocketMessage[PlayerId], 255),
		ctx:      ctx,
		cancel:   cancel,
		wg:       sync.WaitGroup{},
		lastSeen: make(map[PlayerId]time.Time),
	}
	if options.CleanupPeriod == 0 {
		room.cleanupPeriod = time.Second * 30
	} else {
		room.cleanupPeriod = options.CleanupPeriod
	}

	if options.Slogger != nil {
		room.Slogger = options.Slogger.With("room", room.ID)
	} else {
		room.Slogger = slog.Default().With("room", room.ID)
	}
	return room
}

func (room *Room[RoomId, PlayerId]) GetPlayerPresence() []PlayerPresence[PlayerId] {
	room.mu.RLock()
	playerPresences := make([]PlayerPresence[PlayerId], 0, len(room.players))
	for playerId, p := range room.players {
		playerPresences = append(playerPresences, PlayerPresence[PlayerId]{
			ID:          playerId,
			IsConnected: p != nil,
		})
	}
	room.mu.RUnlock()
	return playerPresences
}

func (room *Room[RoomId, PlayerId]) Start() {
	sl := room.Slogger.With("func", "room.Start")
	sl.Debug("starting")
	ticker := time.NewTicker(room.cleanupPeriod)
	defer func() {
		ticker.Stop()
		sl.Info("stopped")
	}()
	for {
		select {
		case <-ticker.C:
			sl.Debug("Cleaning up players")
			room.CleanUpPlayers()
		case <-room.ctx.Done():
			sl.Debug("stopping")
			return
		case msg := <-room.messages:
			sl.Debug("message", "type", msg.Type, "contents", msg.Message)
			switch msg.Type {
			case Disconnect:
				sl.Debug("disconnecting", "player", msg.ReferenceID)
				room.mu.Lock()
				room.players[msg.ReferenceID] = nil
				room.lastSeen[msg.ReferenceID] = time.Now()
				room.mu.Unlock()
				sl.Debug("disconnected", "player", msg.ReferenceID)
				go room.Options.OnDisconnect(msg.ReferenceID)

			case Message:
				sl.Debug("message", "player", msg.ReferenceID)
				go room.Options.OnMessage(msg.ReferenceID, msg.Message)
			}
		}
	}
}

func (room *Room[RoomId, PlayerId]) Stop() {
	sl := room.Slogger.With("func", "room.Stop")
	sl.Debug("closing", "status", "started")
	room.mu.RLock()
	playersToClose := make([]PlayerId, 0, len(room.players))

	for playerId := range room.players {
		playersToClose = append(playersToClose, playerId)
	}
	room.mu.RUnlock()
	for _, playerId := range playersToClose {
		sl.Debug("closing player", "player", playerId)
		playerConn := room.players[playerId]
		if playerConn == nil {
			sl.Debug("player already closed", "player", playerId)
			continue
		}
		playerConn.Close() // should be blocking
		sl.Debug("closed player", "player", playerId)
	}
	close(room.messages)
	sl.Debug("room closed", "status", "completed")
}

func (room *Room[RoomId, PlayerId]) SendMessageToPlayer(player PlayerId, message []byte) {
	sl := room.Slogger.With("func", "room.SendMessageToPlayer")
	sl.Debug("sending message", "player", player, "message", message)
	room.mu.RLock()
	defer room.mu.RUnlock()

	for key, p := range room.players {
		if p == nil {
			continue
		}
		sl.Debug("checking",
			slog.Group("key",
				"value", key,
				"type", fmt.Sprintf("%T", key),
			),
			slog.Group("p.id",
				"value", p.ReferenceID,
				"type", fmt.Sprintf("%T", p.ReferenceID),
			),
		)
		if p.ReferenceID == player {
			p.Send <- message
			return
		}
	}

	ps, ok := room.players[player]
	if !ok {
		sl.Debug("player not found", "player", player)
		return
	}
	ps.Send <- message
}

func (room *Room[RoomId, PlayerId]) SendMessageToAllPlayers(message []byte) {
	sl := room.Slogger.With("func", "room.SendMessageToAllPlayers")
	room.mu.RLock()
	for _, p := range room.players {
		if p == nil {
			continue
		}
		sl.Debug("sending message", "player", p.ReferenceID)
		p.Send <- message
	}
	room.mu.RUnlock()
}

func (room *Room[RoomId, PlayerId]) CleanUpPlayers() {
	sl := room.Slogger.With("func", "room.CleanUpPlayers")
	sl.Debug("starting")
	room.mu.Lock()
	defer room.mu.Unlock()
	for playerId, p := range room.players {
		if p == nil && time.Since(room.lastSeen[playerId]) > room.cleanupPeriod {
			sl.Info("removing", "player", playerId,
				slog.Group("checks",
					"lastSeen", room.lastSeen[playerId],
					"timeSince", time.Since(room.lastSeen[playerId]),
					"cleanupPeriod", room.cleanupPeriod,
					"cleanupPeriodExceeded", time.Since(room.lastSeen[playerId]) > room.cleanupPeriod,
				))
			delete(room.players, playerId)
		}
	}
	sl.Debug("finished")
}
