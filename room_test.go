package goroom

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockSocketSession provides a way to simulate a SocketSession for testing purposes.
type mockSocketSession[PlayerID comparable] struct {
	referenceID  PlayerID
	sentMessages [][]byte
}

func (m *mockSocketSession[PlayerID]) ReferenceID() PlayerID {
	return m.referenceID
}

func (m *mockSocketSession[PlayerID]) Send(message []byte) {
	m.sentMessages = append(m.sentMessages, message)
}

func (m *mockSocketSession[PlayerID]) Close() {}

// newMockSocketSession creates a new mock session for a given player ID.
func newMockSocketSession[PlayerID comparable](playerID PlayerID) *mockSocketSession[PlayerID] {
	return &mockSocketSession[PlayerID]{
		referenceID:  playerID,
		sentMessages: make([][]byte, 0),
	}
}

type mockHandler[PlayerID comparable] struct {
	mu                  sync.RWMutex
	OnConnectResults    []PlayerID
	OnDisconnectResults []PlayerID
	OnMessageResults    []SocketMessage[PlayerID]
	OnRemoveResults     []PlayerID
}

func newMockHandler[PlayerID comparable]() *mockHandler[PlayerID] {
	return &mockHandler[PlayerID]{
		OnConnectResults:    []PlayerID{},
		OnDisconnectResults: []PlayerID{},
		OnMessageResults:    []SocketMessage[PlayerID]{},
		OnRemoveResults:     []PlayerID{},
	}
}
func (mh *mockHandler[PlayerID]) GetOnConnectResults() []PlayerID {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.OnConnectResults
}
func (mh *mockHandler[PlayerID]) GetOnDisconnectResults() []PlayerID {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.OnDisconnectResults
}
func (mh *mockHandler[PlayerID]) GetOnMessageResults() []SocketMessage[PlayerID] {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.OnMessageResults
}
func (mh *mockHandler[PlayerID]) GetOnRemoveResults() []PlayerID {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return mh.OnRemoveResults
}

func (mh *mockHandler[PlayerID]) OnConnect(player PlayerID) {
	mh.mu.Lock()
	mh.OnConnectResults = append(mh.OnConnectResults, player)
	mh.mu.Unlock()
}
func (mh *mockHandler[PlayerID]) OnDisconnect(player PlayerID) {
	mh.mu.Lock()
	mh.OnDisconnectResults = append(mh.OnDisconnectResults, player)
	mh.mu.Unlock()
}
func (mh *mockHandler[PlayerID]) OnMessage(player PlayerID, message []byte) {
	mh.mu.Lock()
	mh.OnMessageResults = append(mh.OnMessageResults, SocketMessage[PlayerID]{
		ReferenceID: player,
		Type:        Message,
		Message:     message,
	})
	mh.mu.Unlock()
}
func (mh *mockHandler[PlayerID]) OnClose(player PlayerID) {
	mh.mu.Lock()
	mh.OnRemoveResults = append(mh.OnRemoveResults, player)
	mh.mu.Unlock()
}

// setupTestRoom initializes a new Room for testing and returns it with a cleanup function.
func setupTestRoom[PlayerID comparable](t *testing.T, roomID string) (*Room[string, PlayerID], *mockHandler[PlayerID], func()) {

	handler := newMockHandler[PlayerID]()

	room := NewRoom[string, PlayerID](context.Background(), roomID, Options[PlayerID]{
		OnConnect:    handler.OnConnect,
		OnDisconnect: handler.OnDisconnect,
		OnMessage:    handler.OnMessage,
		OnRemove:     handler.OnClose,
	})
	if room == nil {
		t.Fatal("NewRoom returned nil")
	}

	// The run method is started in a goroutine, so we need to give it a moment to initialize.
	time.Sleep(10 * time.Millisecond)

	// Return the room and a cleanup function to close it.
	return room, handler, func() {
		t.Log("Stopping room")
		room.Stop()
		t.Log("Stopped room")
	}
}

func TestNewRoom(t *testing.T) {
	t.Run("should create a new room with initial values", func(t *testing.T) {
		roomID := "test-room-1"
		room, mock, cleanup := setupTestRoom[string](t, roomID)
		defer cleanup()

		if room.ID != roomID {
			t.Errorf("expected room ID to be '%s', got '%s'", roomID, room.ID)
		}
		if len(room.players) != 0 {
			t.Errorf("expected initial player count to be 0, got %d", len(room.players))
		}

		connResults := mock.GetOnConnectResults()
		if len(connResults) != 0 {
			t.Errorf("expected onConnect to NOT be called, got %d", len(connResults))
		}

	})
	t.Run("should set the cleanup period", func(t *testing.T) {
		handler := newMockHandler[string]()

		room := NewRoom[string, string](context.Background(), "roomCleanup", Options[string]{
			OnConnect:     handler.OnConnect,
			OnDisconnect:  handler.OnDisconnect,
			OnMessage:     handler.OnMessage,
			OnRemove:      handler.OnClose,
			CleanupPeriod: time.Second,
		})
		if room == nil {
			t.Fatal("NewRoom returned nil")
		}

		// The run method is started in a goroutine, so we need to give it a moment to initialize.
		time.Sleep(10 * time.Millisecond)

		if room.cleanupPeriod != time.Second {
			t.Errorf("expected cleanup period to be %s, got %s", time.Second, room.cleanupPeriod)
		}

		t.Log("Stopping room")
		room.Stop()
		t.Log("Stopped room")
	})
}

func TestRoom_Run_Close(t *testing.T) {
	t.Run("should start the run loop and close it gracefully", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[int](t, "test-room-close")

		go room.Start()

		// The run loop should be active. We can check this by seeing if the done channel is open.
		select {
		case <-room.ctx.Done():
			t.Fatal("room context should not be done immediately after creation")
		default:
			// Context is not done, which is correct.
		}

		cleanup() // This calls room.Close()

		// After closing, the context's Done channel should be closed.
		select {
		case <-room.ctx.Done():
			// This is the expected outcome.
		case <-time.After(1 * time.Second):
			t.Fatal("room did not close within the expected time")
		}
	})
}

func TestRoom_GetPlayerPresences(t *testing.T) {
	t.Run("should return the presence of a player", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()

		players := []string{"player-1", "player-2", "player-3"}
		room.players = make(map[string]SocketSessioner[string])
		for _, p := range players {
			room.players[p] = nil
			room.lastSeen[p] = time.Now()
		}

		presence := room.GetPlayerPresences()

		if len(presence) != len(players) {
			t.Fatalf("expected presence count to be %d, got %d", len(players), len(presence))
		}

		for _, p := range presence {
			if p.IsConnected == true {
				t.Fatalf("expected presence to be false for player %s", p.ID)
			}
			if p.LastSeen.IsZero() {
				t.Fatalf("expected presence to have a LastSeen value for player %s", p.ID)
			}
		}

	})
}

func TestRoom_SendMessageToPlayer(t *testing.T) {
	t.Run("should send message to a player", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()

		players := []string{"player-1", "player-2", "player-3"}
		room.players = make(map[string]SocketSessioner[string])
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}

		tstMessage := []byte("test message")

		room.SendMessageToPlayer(players[0], tstMessage)
		room.SendMessageToPlayer(players[0], tstMessage)
		room.SendMessageToPlayer(players[1], tstMessage)

		if len(room.players[players[0]].(*mockSocketSession[string]).sentMessages) != 2 {
			t.Errorf("expected %s to receive 2 messages, got %d", players[0], len(room.players[players[0]].(*mockSocketSession[string]).sentMessages))
		}
		if len(room.players[players[1]].(*mockSocketSession[string]).sentMessages) != 1 {
			t.Errorf("expected %s to receive 1 message, got %d", players[1], len(room.players[players[1]].(*mockSocketSession[string]).sentMessages))
		}
		if len(room.players[players[2]].(*mockSocketSession[string]).sentMessages) != 0 {
			t.Errorf("expected %s to receive 0 messages, got %d", players[2], len(room.players[players[2]].(*mockSocketSession[string]).sentMessages))
		}
	})
}

func TestRoom_GetPlayerPresence(t *testing.T) {
	t.Run("should return the presence of a player", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		room.players = make(map[string]SocketSessioner[string])

		p1Session := newMockSocketSession[string](players[0])
		room.players[players[0]] = p1Session
		room.players[players[1]] = nil

		if !room.GetPlayerPresence(players[0]).IsConnected {
			t.Errorf("expected presence to be true for player %s", players[0])
		}
		if room.GetPlayerPresence(players[1]).IsConnected {
			t.Errorf("expected presence to be false for player %s", players[1])
		}
		if room.GetPlayerPresence(players[2]).IsConnected {
			t.Errorf("expected presence to be false for player %s", players[2])
		}
	})
}

func TestRoom_SendMessageToAllPlayers(t *testing.T) {
	t.Run("should send message to all players", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()

		players := []string{"player-1", "player-2", "player-3"}
		room.players = make(map[string]SocketSessioner[string])
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}

		tstMessage := []byte("test message")

		room.SendMessageToAllPlayers(tstMessage)
		room.SendMessageToAllPlayers(tstMessage)

		if len(room.players[players[0]].(*mockSocketSession[string]).sentMessages) != 2 {
			t.Errorf("expected %s to receive 2 messages, got %d", players[0], len(room.players[players[0]].(*mockSocketSession[string]).sentMessages))
		}
		if len(room.players[players[1]].(*mockSocketSession[string]).sentMessages) != 2 {
			t.Errorf("expected %s to receive 2 messages, got %d", players[1], len(room.players[players[1]].(*mockSocketSession[string]).sentMessages))
		}
		if len(room.players[players[2]].(*mockSocketSession[string]).sentMessages) != 2 {
			t.Errorf("expected %s to receive 2 messages, got %d", players[2], len(room.players[players[2]].(*mockSocketSession[string]).sentMessages))
		}

	})
}

func TestRoom_CleanUpPlayers(t *testing.T) {
	t.Run("should remove disconnected players from the room", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()

		players := []string{"player-1", "player-2", "player-3", "player-4", "player-5", "player-6", "player-7", "player-8", "player-9"}
		offsets := []int{-70, -60, -50, -40, -30, -20, -10, -5, -1}

		room.players = make(map[string]SocketSessioner[string])
		for idx, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
			room.lastSeen[p] = time.Now().Add(time.Duration(offsets[idx]) * time.Second)
		}

		room.CleanUpPlayers()

		count := 0
		for playerID := range room.players {
			if room.players[playerID] == nil {
				count++
			}
		}
		expectedCount := 0
		for _, offset := range offsets {
			if time.Duration(offset) >= defaultCleanupPeriod {
				expectedCount++
			}
		}

		if count != expectedCount {
			t.Errorf("expected %d players to be removed from the room, got %d", expectedCount, count)
		}
	})
}

func TestRoom_SetRoomId(t *testing.T) {
	t.Run("should set the room ID", func(t *testing.T) {
		ctx := context.Background()
		room := NewRoom[string, string](ctx, "initial-id", Options[string]{})

		newID := "new-id"
		room.SetRoomID(newID)

		if room.ID != newID {
			t.Errorf("expected room ID to be %s, but got %s", newID, room.ID)
		}
	})
	t.Run("should update the slogger", func(t *testing.T) {

		var buf strings.Builder
		sl := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		ctx := context.Background()
		initID := "initial-id"
		room := NewRoom[string, string](ctx, initID, Options[string]{
			Slogger: sl,
		})

		newID := "new-id"
		room.SetRoomID(newID)

		// Do something to trigger log output
		room.CleanUpPlayers()

		logOutput := buf.String()
		if strings.Contains(logOutput, initID) {
			t.Errorf("expected log output to not contain %s, but got %s", initID, logOutput)
		}
		if !strings.Contains(logOutput, newID) {
			t.Errorf("expected log output to contain %s, but got %s", newID, logOutput)
		}
	})

	t.Run("should not update if room has already been started", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		initID := "initial-id"
		room := NewRoom[string, string](ctx, initID, Options[string]{})

		go room.Start()

		// let's wait for the go routine to do something
		time.Sleep(10 * time.Millisecond)

		newID := "new-id"
		room.SetRoomID(newID)

		if room.ID != initID {
			t.Errorf("expected room ID to be %s, but got %s", initID, room.ID)
		}
		if room.ctx.Err() != nil {
			t.Errorf("expected room context to not be done, but got done")
		}
	})
}

func TestRoom_SetPlayers(t *testing.T) {
	t.Run("should set the players in the room", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}
		if len(room.players) != len(players) {
			t.Errorf("expected %d players to be set, got %d", len(players), len(room.players))
		}
	})
	t.Run("should remove players that are not in the new list", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}

		room.SetPlayers(players[:1])
		if len(room.players) != 1 {
			t.Errorf("expected %d players to be set, got %d", 1, len(room.players))
		}
	})
	t.Run("should not remove players that are in the new list", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}
		room.SetPlayers(players)
		if len(room.players) != len(players) {
			t.Errorf("expected %d players to be set, got %d", len(players), len(room.players))
		}
	})
	t.Run("should not remove players that are in the new list and have a different ID", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}
		room.SetPlayers(append(players, "player-4"))
		if len(room.players) != len(players)+1 {
			t.Errorf("expected %d players to be set, got %d", len(players)+1, len(room.players))
		}
	})
	t.Run("should not remove players that are in the new list and have the same ID", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}
		room.SetPlayers(append(players, players[0]))
	})
	t.Run("should not remove players that are in the new list and have the same ID and are not in the old list", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			ss := newMockSocketSession[string](p)
			room.players[p] = ss
		}
		room.SetPlayers(append(players, players[0], players[1]))
	})
	t.Run("should close any open connections to players that are not in the new list", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()
		players := []string{"player-1", "player-2", "player-3"}
		for _, p := range players {
			room.players[p] = newMockSocketSession[string](p)
		}
		room.SetPlayers(players[:1])
		if len(room.players) != 1 {
			t.Errorf("expected %d players to be set, got %d", 1, len(room.players))
		}
		if room.players[players[1]] != nil {
			t.Errorf("expected player %s to be closed, but it is still open", players[1])
		}
	})
}
