package room

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// setupTestSession creates a new SocketSession with a mocked network connection (net.Pipe)
// for testing purposes. It returns the session, the client-side of the connection,
// and the messages channel. It also handles cleanup using t.Cleanup.
func setupTestSession[PlayerId comparable](t *testing.T, referenceID PlayerId) (*SocketSession[PlayerId], net.Conn, chan SocketMessage[PlayerId]) {
	serverConn, clientConn := net.Pipe()
	messages := make(chan SocketMessage[PlayerId], 10) // Buffered channel to avoid blocking in tests
	session := NewSocketSession(serverConn, referenceID, messages)

	// Close the session and connections when the test is done
	t.Cleanup(func() {
		session.Close()
		_ = clientConn.Close()
	})

	return session, clientConn, messages
}

func TestNewSocketSession(t *testing.T) {
	t.Run("should create a new session with all fields initialized", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer func() {
			_ = serverConn.Close()
		}()
		defer func() {
			_ = clientConn.Close()
		}()

		messages := make(chan SocketMessage[string], 1)
		referenceID := "player1"

		session := NewSocketSession(serverConn, referenceID, messages)

		if session.referenceID != referenceID {
			t.Errorf("Expected referenceID to be %v, got %v", referenceID, session.referenceID)
		}
		if session.conn == nil {
			t.Error("Expected conn to be non-nil")
		}
		if session.send == nil {
			t.Error("Expected send channel to be non-nil")
		}
		if session.Messages == nil {
			t.Error("Expected Messages channel to be non-nil")
		}
		if session.ctx == nil {
			t.Error("Expected ctx to be non-nil")
		}
		if session.cancel == nil {
			t.Error("Expected cancel function to be non-nil")
		}

		// Properly close the session to ensure goroutines exit and avoid test leaks.
		session.Close()
	})
}

func TestSocketSession_ReadLoop(t *testing.T) {
	t.Run("should read a message and forward it to the messages channel", func(t *testing.T) {
		_, clientConn, messages := setupTestSession(t, "player1")

		// Simulate a client sending a message
		testMessage := []byte("hello from client")
		if err := wsutil.WriteClientBinary(clientConn, testMessage); err != nil {
			t.Fatalf("Failed to write client message: %v", err)
		}

		// Assert that the message is received on the Messages channel
		select {
		case msg := <-messages:
			if msg.Type != Message {
				t.Errorf("Expected message type to be Message, got %v", msg.Type)
			}
			if !bytes.Equal(msg.Message, testMessage) {
				t.Errorf("Expected message to be '%s', got '%s'", testMessage, msg.Message)
			}
			if msg.ReferenceID != "player1" {
				t.Errorf("Expected referenceID to be 'player1', got '%v'", msg.ReferenceID)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for message from ReadLoop")
		}
	})

	t.Run("should send a disconnect message on connection close", func(t *testing.T) {
		_, clientConn, messages := setupTestSession(t, "player2")

		// Close the client side of the pipe to trigger the ReadLoop to exit
		_ = clientConn.Close()

		// Assert that a disconnect message is received
		select {
		case msg := <-messages:
			if msg.Type != Disconnect {
				t.Errorf("Expected message type to be Disconnect, got %v", msg.Type)
			}
			if msg.ReferenceID != "player2" {
				t.Errorf("Expected referenceID to be 'player2', got '%v'", msg.ReferenceID)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for disconnect message")
		}
	})
}

func TestSocketSession_WriteLoop(t *testing.T) {
	t.Run("should write a message from the send channel to the connection", func(t *testing.T) {
		session, clientConn, _ := setupTestSession(t, "player3")

		// send a message to the session's send channel
		testMessage := []byte("hello from server")
		session.send <- testMessage

		// Read the message from the client side of the pipe to confirm it was sent
		msg, op, err := wsutil.ReadServerData(clientConn)
		if err != nil {
			t.Fatalf("Failed to read server data from pipe: %v", err)
		}

		if op != ws.OpBinary {
			t.Errorf("Expected OpCode to be OpBinary, got %v", op)
		}
		if !bytes.Equal(msg, testMessage) {
			t.Errorf("Expected message to be '%s', got '%s'", testMessage, msg)
		}
	})
}

func TestSocketSession_Close(t *testing.T) {
	t.Run("should close the connection and wait for loops to exit", func(t *testing.T) {
		session, clientConn, _ := setupTestSession(t, "player4")

		// This WaitGroup helps confirm that the underlying connection was indeed closed,
		// which would cause the blocking Read call to return an error.
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This read will block until the connection is closed by session.Close()
			_, _ = clientConn.Read(make([]byte, 1))
		}()

		// Close the session, which should close the connection and exit the loops
		session.Close()

		// Wait for the reading goroutine to finish. If it doesn't finish, it means
		// the connection wasn't closed and the test will time out.
		waitChan := make(chan struct{})
		go func() {
			wg.Wait()
			close(waitChan)
		}()

		select {
		case <-waitChan:
			// Success, the goroutine finished as expected.
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for connection to close. session.Close() may not be working correctly.")
		}
	})
}

func Test_unregisterMessage(t *testing.T) {
	t.Run("should create a correct disconnect message", func(t *testing.T) {
		// This tests the non-exported helper function
		session := &SocketSession[string]{
			referenceID: "test-player",
		}

		msg := session.unregisterMessage()

		if msg.Type != Disconnect {
			t.Errorf("Expected message type to be Disconnect, got %v", msg.Type)
		}
		if msg.ReferenceID != "test-player" {
			t.Errorf("Expected referenceID to be 'test-player', got '%v'", msg.ReferenceID)
		}
		if msg.Message != nil {
			t.Errorf("Expected Message field to be nil, got %v", msg.Message)
		}
	})
}
