package goroom

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

type SocketSessioner[PlayerID comparable] interface {
	ReferenceID() PlayerID
	Send(message []byte)
	Close()
}

type Room[RoomId comparable, PlayerID comparable] struct {
	ID   RoomId
	opts Options[PlayerID]

	mu            sync.RWMutex
	Status        RoomStatus
	players       map[PlayerID]SocketSessioner[PlayerID]
	lastSeen      map[PlayerID]time.Time
	cleanupPeriod time.Duration

	// MessageProcessing
	messages chan SocketMessage[PlayerID]

	// Concurrency
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Logging
	Slogger *slog.Logger
}

type Options[PlayerID comparable] struct {
	OnConnect    func(player PlayerID)
	OnDisconnect func(player PlayerID)
	OnRemove     func(player PlayerID)
	OnMessage    func(player PlayerID, message []byte)

	CleanupPeriod time.Duration

	Slogger *slog.Logger
}

const defaultCleanupPeriod time.Duration = time.Second * 30

func NewRoom[RoomId comparable, PlayerID comparable](parentCtx context.Context, id RoomId, options Options[PlayerID]) *Room[RoomId, PlayerID] {
	ctx, cancel := context.WithCancel(parentCtx)
	room := &Room[RoomId, PlayerID]{
		ID:       id,
		opts:     options,
		Status:   Open,
		players:  make(map[PlayerID]SocketSessioner[PlayerID]), //*SocketSession[PlayerID]),
		messages: make(chan SocketMessage[PlayerID], 255),
		ctx:      ctx,
		cancel:   cancel,
		wg:       sync.WaitGroup{},
		lastSeen: make(map[PlayerID]time.Time),
	}
	if options.CleanupPeriod == 0 {
		room.cleanupPeriod = defaultCleanupPeriod
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

func (room *Room[RoomId, PlayerID]) GetPlayerPresence() []PlayerPresence[PlayerID] {
	room.mu.RLock()
	playerPresences := make([]PlayerPresence[PlayerID], 0, len(room.players))
	for playerID, p := range room.players {
		playerPresences = append(playerPresences, PlayerPresence[PlayerID]{
			ID:          playerID,
			IsConnected: p != nil,
		})
	}
	room.mu.RUnlock()
	return playerPresences
}

func (room *Room[RoomId, PlayerID]) Start() {
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
				go room.opts.OnDisconnect(msg.ReferenceID)

			case Message:
				sl.Debug("message", "player", msg.ReferenceID)
				go room.opts.OnMessage(msg.ReferenceID, msg.Message)
			}
		}
	}
}

func (room *Room[RoomId, PlayerID]) Stop() {
	sl := room.Slogger.With("func", "room.Stop")
	sl.Debug("closing", "status", "started")
	room.mu.RLock()
	playersToClose := make([]PlayerID, 0, len(room.players))

	for playerID := range room.players {
		playersToClose = append(playersToClose, playerID)
	}
	room.mu.RUnlock()
	for _, playerID := range playersToClose {
		sl.Debug("closing player", "player", playerID)
		playerConn := room.players[playerID]
		if playerConn == nil {
			sl.Debug("player already closed", "player", playerID)
			continue
		}
		playerConn.Close() // should be blocking
		sl.Debug("closed player", "player", playerID)
	}
	close(room.messages)
	room.cancel()
	sl.Debug("room closed", "status", "completed")
}

func (room *Room[RoomId, PlayerID]) SendMessageToPlayer(player PlayerID, message []byte) {
	sl := room.Slogger.With("func", "room.SendMessageToPlayer")
	sl.Debug("sending message", "player", player, "message", message)
	room.mu.RLock()
	defer room.mu.RUnlock()

	ps, ok := room.players[player]
	if !ok {
		sl.Debug("player not found", "player", player)
		return
	}
	ps.Send(message)
}

func (room *Room[RoomId, PlayerID]) SendMessageToAllPlayers(message []byte) {
	sl := room.Slogger.With("func", "room.SendMessageToAllPlayers")
	room.mu.RLock()
	for _, p := range room.players {
		if p == nil {
			continue
		}
		sl.Debug("sending message", "player", p.ReferenceID())
		p.Send(message)
	}
	room.mu.RUnlock()
}

func (room *Room[RoomId, PlayerID]) CleanUpPlayers() {
	if room.Status != Open {
		return
	}
	sl := room.Slogger.With("func", "room.CleanUpPlayers")
	sl.Debug("starting")
	room.mu.Lock()
	defer room.mu.Unlock()

	for playerID, p := range room.players {
		if p == nil && time.Since(room.lastSeen[playerID]) > room.cleanupPeriod {
			sl.Info("removing", "player", playerID,
				slog.Group("checks",
					"lastSeen", room.lastSeen[playerID],
					"timeSince", time.Since(room.lastSeen[playerID]),
					"cleanupPeriod", room.cleanupPeriod,
					"cleanupPeriodExceeded", time.Since(room.lastSeen[playerID]) > room.cleanupPeriod,
				))
			delete(room.players, playerID)
			go func(pid PlayerID) {
				if room.opts.OnRemove == nil {
					return
				}
				room.opts.OnRemove(pid)
			}(playerID)
		}
	}

	sl.Debug("finished")
}

func (room *Room[RoomId, PlayerID]) SetStatus(status RoomStatus) {
	if room.Status == status {
		return
	}
	sl := room.Slogger.With("func", "room.SetStatus")
	sl.Debug("setting status", "status", status)
	room.mu.Lock()
	defer room.mu.Unlock()
	room.Status = status
	if status == Locked {
		// Remove disconnected players
		for pid, p := range room.players {
			if p != nil {
				continue
			}
			// Remove reference to previously connected players
			delete(room.players, pid)
			delete(room.lastSeen, pid)
			go room.opts.OnRemove(pid)
		}
	}
}

func (room *Room[RoomId, PlayerID]) SetPlayers(players []PlayerID) error {
	room.mu.Lock()
	defer room.mu.Unlock()
	for _, pid := range players {
		_, ok := room.players[pid]
		if ok {
			continue
		}
		room.players[pid] = nil
	}

	//playersToRemove := make([]PlayerID, 0)
	for pid, ss := range room.players {
		if !slices.Contains(players, pid) {
			if ss == nil {
				continue
			}
			ss.Close()
			room.players[pid] = nil
			delete(room.players, pid)
			delete(room.lastSeen, pid)
			go room.opts.OnRemove(pid)
			//playersToRemove = append(playersToRemove, pid)
		}
	}

	return nil
}
