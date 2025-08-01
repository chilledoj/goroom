package goroom

import (
	"context"
	"testing"
	"time"
)

// mockSocketSession provides a way to simulate a SocketSession for testing purposes.
type mockSocketSession[PlayerId comparable] struct {
	ReferenceID  PlayerId
	sentMessages [][]byte
}

func (m *mockSocketSession[PlayerId]) ReferenceId() PlayerId {
	return m.ReferenceID
}

func (m *mockSocketSession[PlayerId]) Send(message []byte) {
	m.sentMessages = append(m.sentMessages, message)
	return
}

func (m *mockSocketSession[PlayerId]) Close() {}

// newMockSocketSession creates a new mock session for a given player ID.
func newMockSocketSession[PlayerId comparable](playerID PlayerId) *mockSocketSession[PlayerId] {
	return &mockSocketSession[PlayerId]{
		ReferenceID:  playerID,
		sentMessages: make([][]byte, 0),
	}
}

type mockHandler[PlayerId comparable] struct {
	OnConnectResults    []PlayerId
	OnDisconnectResults []PlayerId
	OnMessageResults    []SocketMessage[PlayerId]
}

func newMockHandler[PlayerId comparable]() *mockHandler[PlayerId] {
	return &mockHandler[PlayerId]{
		OnConnectResults:    []PlayerId{},
		OnDisconnectResults: []PlayerId{},
		OnMessageResults:    []SocketMessage[PlayerId]{},
	}
}

func (mh *mockHandler[PlayerId]) OnConnect(player PlayerId) {
	mh.OnConnectResults = append(mh.OnConnectResults, player)
}
func (mh *mockHandler[PlayerId]) OnDisconnect(player PlayerId) {
	mh.OnDisconnectResults = append(mh.OnDisconnectResults, player)
}
func (mh *mockHandler[PlayerId]) OnMessage(player PlayerId, message []byte) {
	mh.OnMessageResults = append(mh.OnMessageResults, SocketMessage[PlayerId]{
		ReferenceID: player,
		Type:        Message,
		Message:     message,
	})
}

// setupTestRoom initializes a new Room for testing and returns it with a cleanup function.
func setupTestRoom[PlayerId comparable](t *testing.T, roomID string) (*Room[string, PlayerId], *mockHandler[PlayerId], func()) {

	handler := newMockHandler[PlayerId]()

	room := NewRoom[string, PlayerId](context.Background(), roomID, Options[PlayerId]{
		OnConnect:    handler.OnConnect,
		OnDisconnect: handler.OnDisconnect,
		OnMessage:    handler.OnMessage,
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
		roomId := "test-room-1"
		room, mock, cleanup := setupTestRoom[string](t, roomId)
		defer cleanup()

		if room.ID != roomId {
			t.Errorf("expected room ID to be '%s', got '%s'", roomId, room.ID)
		}
		if len(room.players) != 0 {
			t.Errorf("expected initial player count to be 0, got %d", len(room.players))
		}

		if len(mock.OnConnectResults) != 0 {
			t.Errorf("expected onConnect to NOT be called, got %d", len(mock.OnConnectResults))
		}

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

func TestRoom_GetPlayerPresence(t *testing.T) {
	t.Run("should return the presence of a player", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-presence")
		defer cleanup()

		players := []string{"player-1", "player-2", "player-3"}
		room.players = make(map[string]SocketSessioner[string])
		for _, p := range players {
			room.players[p] = nil
		}

		presence := room.GetPlayerPresence()

		if len(presence) != len(players) {
			t.Fatalf("expected presence count to be %d, got %d", len(players), len(presence))
		}

		for _, p := range presence {
			if p.IsConnected == true {
				t.Fatalf("expected presence to be false for player %s", p.ID)
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
		for playerId := range room.players {
			if room.players[playerId] == nil {
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

/*
func TestRoom_HandleJoinLeave(t *testing.T) {
	t.Run("should handle a player joining and leaving", func(t *testing.T) {
		room, _, cleanup := setupTestRoom[string](t, "test-room-join-leave")
		defer cleanup()

		// --- Setup two mock players ---
		player1 := "player-1"
		player2 := "player-2"
		session1 := newMockSocketSession(player1)
		session2 := newMockSocketSession(player2)

		// --- Player 1 Joins ---
		room.register <- &sessionRegistration[string, string]{
			player:  player1,
			session: session1,
		}
		time.Sleep(50 * time.Millisecond) // Give the room time to process the join.

		// Assert that player 1 is in the room.
		if _, ok := room.players[player1.ID]; !ok {
			t.Fatal("player 1 should have been added to the room's player list")
		}
		if _, ok := room.sessions[player1.ID]; !ok {
			t.Fatal("player 1's session should have been added to the room's session list")
		}

		// --- Player 2 Joins ---
		room.register <- &sessionRegistration[string, string]{
			player:  player2,
			session: session2,
		}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// Assert that player 2 is now in the room.
		if len(room.players) != 2 {
			t.Fatalf("expected player count to be 2, got %d", len(room.players))
		}

		// Assert that player 1 received a notification about player 2 joining.
		select {
		case msg := <-session1.SendChan:
			// This is an oversimplified check. A real implementation would parse the JSON.
			if !bytes.Contains(msg, []byte("player_update")) || !bytes.Contains(msg, []byte(player2.ID)) {
				t.Errorf("player 1 did not receive a proper join notification for player 2: %s", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for player 2 join notification")
		}

		// --- Player 1 Leaves ---
		room.messages <- SocketMessage[string]{
			referenceID: player1,
			Type:        Disconnect,
		}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// Assert that player 1 is removed.
		if len(room.players) != 1 {
			t.Fatalf("expected player count to be 1 after a player leaves, got %d", len(room.players))
		}
		if _, ok := room.players[player1]; ok {
			t.Fatal("player 1 should have been removed from the room's player list")
		}
		if session1.Closed != true {
			t.Error("player 1's session should be closed after leaving")
		}

		// Assert that player 2 received a notification about player 1 leaving.
		select {
		case msg := <-session2.SendChan:
			if !bytes.Contains(msg, []byte("player_update")) || !bytes.Contains(msg, []byte(player1.ID)) {
				// This is a weak check; ideally, you'd unmarshal and check the player list length.
				t.Logf("Received leave notification: %s", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for player 1 leave notification")
		}
	})
}

func TestRoom_HandleBroadcast(t *testing.T) {
	t.Run("should broadcast a message to all other players", func(t *testing.T) {
		room, cleanup := setupTestRoom(t, "test-room-broadcast")
		defer cleanup()

		// --- Setup three mock players ---
		players := []*Player[string]{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}}
		sessions := make(map[string]*mockSocketSession)
		for _, p := range players {
			session := newMockSocketSession(p.ID)
			sessions[p.ID] = session
			room.register <- &sessionRegistration[string, string]{player: p, session: session}
		}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// --- send a broadcast message from player 1 ---
		broadcastMessage := "hello everyone!"
		room.messages <- SocketMessage[string]{
			referenceID: players[0].ID,
			Type:        Broadcast,
			Payload:     broadcastMessage,
		}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// --- Check that player 2 and player 3 received the broadcast message ---
		for _, p := range players[1:] {
			select {
			case msg := <-sessions[p.ID].SendChan:
				if !bytes.Contains(msg, []byte(broadcastMessage)) {
					t.Errorf("player %s did not receive the broadcast message", p.ID)
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatalf("timed out waiting for broadcast message for player %s", p.ID)
			}
		}

		// Ensure the original sender did not receive the message.
		select {
		case msg := <-sessions[players[0].ID].SendChan:
			t.Fatalf("original sender received the broadcast message: %s", msg)
		case <-time.After(10 * time.Millisecond):
			// This is the expected outcome.
		}
	})
}

func TestRoom_HandleDirectMessage(t *testing.T) {
	t.Run("should send a direct message to the intended recipient", func(t *testing.T) {
		room, cleanup := setupTestRoom(t, "test-room-direct-message")
		defer cleanup()

		// --- Setup two mock players ---
		player1 := &Player[string]{ID: "sender"}
		player2 := &Player[string]{ID: "receiver"}
		session1 := newMockSocketSession(player1.ID)
		session2 := newMockSocketSession(player2.ID)
		room.register <- &sessionRegistration[string, string]{player: player1, session: session1}
		room.register <- &sessionRegistration[string, string]{player: player2, session: session2}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// --- send a direct message from player 1 to player 2 ---
		directMessage := "hello receiver!"
		room.messages <- SocketMessage[string]{
			referenceID: player1.ID,
			Type:        DirectMessage,
			TargetID:    player2.ID,
			Payload:     directMessage,
		}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// --- Check that player 2 received the direct message ---
		select {
		case msg := <-session2.SendChan:
			if !bytes.Contains(msg, []byte(directMessage)) {
				t.Errorf("player 2 did not receive the direct message")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for direct message for player 2")
		}

		// --- Ensure the original sender did not receive the message ---
		select {
		case msg := <-session1.SendChan:
			t.Fatalf("original sender received the direct message: %s", msg)
		case <-time.After(10 * time.Millisecond):
			// This is the expected outcome.
		}
	})

	t.Run("should handle direct message to a non-existent player", func(t *testing.T) {
		room, cleanup := setupTestRoom(t, "test-room-direct-message-nonexistent")
		defer cleanup()

		// --- Setup one mock player ---
		player1 := &Player[string]{ID: "sender"}
		session1 := newMockSocketSession(player1.ID)
		room.register <- &sessionRegistration[string, string]{player: player1, session: session1}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// --- send a direct message from player 1 to a non-existent player ---
		directMessage := "hello nonexistent!"
		room.messages <- SocketMessage[string]{
			referenceID: player1.ID,
			Type:        DirectMessage,
			TargetID:    "nonexistent",
			Payload:     directMessage,
		}
		time.Sleep(50 * time.Millisecond) // Give time for processing.

		// --- Ensure the original sender did not receive an error message
		select {
		case msg := <-session1.SendChan:
			t.Fatalf("original sender received an unexpected message: %s", msg)
		case <-time.After(10 * time.Millisecond):
			// This is the expected outcome.  Message should be dropped without error to sender
		}
	})
}

*/
