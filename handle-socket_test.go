package goroom

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	httptest2 "github.com/getlantern/httptest"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoom_HandleSocketWithPlayer(t *testing.T) {
	t.Run("should error on nil player id reference", func(t *testing.T) {

		testW := httptest.NewRecorder()
		testR := httptest.NewRequest("GET", "/", nil)
		var player string
		handler := newMockHandler[string]()

		room := NewRoom[string, string](context.Background(), "test-socket-error", Options[string]{
			OnConnect:    handler.OnConnect,
			OnDisconnect: handler.OnDisconnect,
			OnMessage:    handler.OnMessage,
		})
		if room == nil {
			t.Fatal("NewRoom returned nil")
		}

		var httpErr error

		httpHandler := func(w http.ResponseWriter, r *http.Request) {
			room.HandleSocketWithPlayer(player, func(w http.ResponseWriter, r *http.Request, err error) {
				httpErr = err
				http.Error(w, "error", http.StatusInternalServerError)
			})(w, r)
		}
		httpHandler(testW, testR)
		resp := testW.Result()

		t.Log(resp.StatusCode)
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status code to be %d, got %d", http.StatusInternalServerError, resp.StatusCode)
		}
		if httpErr == nil {
			t.Fatal("httpErr is nil")
		}

	})
	t.Run("should connect player to the room", func(t *testing.T) {
		//var buf []byte
		testW := httptest2.NewRecorder(nil)
		testR := httptest.NewRequest("GET", "/", nil)
		testR.Header.Set("Upgrade", "websocket")
		testR.Header.Set("Connection", "Upgrade")

		testR.Header.Set("Sec-WebSocket-Version", "13")

		key, err := generateChallengeKey()
		if err != nil {
			t.Fatal(err)
		}
		testR.Header.Set("Sec-WebSocket-Key", key)
		testR.Header.Set("Sec-WebSocket-Accept", "13")

		player := "player-1"
		handler := newMockHandler[string]()

		room := NewRoom[string, string](context.Background(), "test-socket-connect", Options[string]{
			OnConnect:    handler.OnConnect,
			OnDisconnect: handler.OnDisconnect,
			OnMessage:    handler.OnMessage,
		})
		if room == nil {
			t.Fatal("NewRoom returned nil")
		}
		defer room.Stop()

		go room.Start()

		var httpErr error

		httpHandler := func(w http.ResponseWriter, r *http.Request) {
			room.HandleSocketWithPlayer(player, func(w http.ResponseWriter, r *http.Request, err error) {
				httpErr = err
				t.Log(err)
				http.Error(w, "error", http.StatusInternalServerError)
			})(w, r)
		}
		httpHandler(testW, testR)
		resp := testW.Result()

		t.Log(resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code to be %d, got %d", http.StatusOK, resp.StatusCode)
		}

		// Got to wait for the go routine to run the OnConnect call
		<-time.After(time.Millisecond * 10)

		if len(handler.OnConnectResults) != 1 {
			t.Fatalf("expected onConnect to be called once, got %d", len(handler.OnConnectResults))
		}
		if handler.OnConnectResults[0] != player {
			t.Fatalf("expected onConnect to be called with player '%s', got '%s'", player, handler.OnConnectResults[0])
		}

		if len(room.players) != 1 {
			t.Fatalf("expected player count to be 1, got %d", len(room.players))
		}

		if httpErr != nil {
			t.Fatalf("expected http errors to be nil, got %s", httpErr.Error())
		}
	})
}

func generateChallengeKey() (string, error) {
	p := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, p); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(p), nil
}

func TestRoom_CanJoin(t *testing.T) {
	t.Run("should return true if room is open", func(t *testing.T) {
		roomId := "test-room-1"
		room, _, cleanup := setupTestRoom[string](t, roomId)
		defer cleanup()

		if !room.CanJoin("player-1") {
			t.Fatal("expected CanJoin to return true")
		}
	})
	t.Run("should return false if room is inactive", func(t *testing.T) {
		roomId := "test-room-2"
		room, _, cleanup := setupTestRoom[string](t, roomId)
		defer cleanup()
		room.Status = Inactive
		if room.CanJoin("player-1") {
			t.Fatal("expected CanJoin to return false")
		}
	})
	t.Run("should return false if room is locked and is new player", func(t *testing.T) {
		roomId := "test-room-2"
		room, _, cleanup := setupTestRoom[string](t, roomId)
		defer cleanup()

		room.Status = Locked

		if room.CanJoin("player-1") {
			t.Fatal("expected CanJoin to return false")
		}
	})
	t.Run("should return true if room is locked and is existing disconnected player", func(t *testing.T) {
		roomId := "test-room-2"
		room, _, cleanup := setupTestRoom[string](t, roomId)
		defer cleanup()

		p1 := "player-1"

		room.players[p1] = nil
		room.lastSeen[p1] = time.Now()

		room.Status = Locked
		if !room.CanJoin(p1) {
			t.Fatal("expected CanJoin to return true")
		}
	})
	t.Run("should return false if player is already connected", func(t *testing.T) {
		roomId := "test-room-3"
		room, _, cleanup := setupTestRoom[string](t, roomId)
		defer cleanup()

		p1 := "player-1"

		ss := newMockSocketSession[string](p1)
		room.players[p1] = ss

		roomStatuses := []RoomStatus{Open, Locked}
		for _, rs := range roomStatuses {
			room.Status = rs
			if room.CanJoin(p1) {
				t.Fatal("expected CanJoin to return false")
			}
		}
	})
}
