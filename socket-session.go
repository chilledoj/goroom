package goroom

import (
	"context"
	"errors"
	"fmt"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"log/slog"
	"net"
	"sync"
	"time"
)

type SocketMessageType int

const (
	Disconnect SocketMessageType = iota - 1
	_
	Message
)

type SocketMessage[PlayerId comparable] struct {
	ReferenceID PlayerId
	Type        SocketMessageType
	Message     []byte
}

type SocketSession[PlayerId comparable] struct {
	// The key bit - the web-socket connection
	conn net.Conn
	// The reference bit
	referenceID PlayerId

	// The message bit
	send     chan []byte
	Messages chan SocketMessage[PlayerId]

	// The concurrency bit
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewSocketSession[PlayerId comparable](conn net.Conn, referenceID PlayerId, messages chan SocketMessage[PlayerId]) *SocketSession[PlayerId] {
	ctx, cancel := context.WithCancel(context.Background())
	s := &SocketSession[PlayerId]{
		conn:        conn,
		referenceID: referenceID,
		send:        make(chan []byte, 255),
		Messages:    messages,
		ctx:         ctx,
		cancel:      cancel,
		wg:          sync.WaitGroup{},
	}

	// START
	s.wg.Add(1)
	go func() {
		s.ReadLoop()
		s.wg.Done()
	}()
	s.wg.Add(1)
	go func() {
		s.WriteLoop()
		s.wg.Done()
	}()
	return s
}

func (s *SocketSession[PlayerId]) ReferenceID() PlayerId {
	return s.referenceID
}

func (s *SocketSession[PlayerId]) Close() {
	s.cancel()
	s.conn.Close()
	s.wg.Wait()
}

func (s *SocketSession[PlayerId]) ReadLoop() {
	sl := slog.With("func", "socket.ReadLoop")
	sl.Debug("starting", "referenceID", s.referenceID)
	defer func() {
		s.conn.Close()
		s.cancel()
		sl.Debug("ReadLoop exited", "referenceID", s.referenceID)
	}()
	for {
		msg, _, err := wsutil.ReadClientData(s.conn)
		if err != nil {
			var er wsutil.ClosedError
			if errors.As(err, &er) {
				sl.Debug("ReadLoop closing", "referenceID", s.referenceID, "reason", er.Reason)
			} else {
				sl.Error("ReadLoop error", "referenceID", s.referenceID, "err", err)
			}
			// send the disconnect message for ANY error that terminates the loop.
			s.Messages <- s.unregisterMessage()
			return
		}
		sl.Debug("ReadLoop message", "referenceID", s.referenceID, "message", fmt.Sprintf("%v", msg))

		sm := SocketMessage[PlayerId]{
			ReferenceID: s.referenceID,
			Type:        Message,
			Message:     msg,
		}

		s.Messages <- sm
		sl.Debug("ReadLoop message sent", "referenceID", s.referenceID, "socket message", fmt.Sprintf("%v", sm))
	}
}

func (s *SocketSession[PlayerId]) WriteLoop() {
	sl := slog.With("func", "socket.WriteLoop")
	sl.Debug("starting", "referenceID", s.referenceID)
	ticker := time.NewTicker(time.Second * 10)
	defer func() {
		ticker.Stop()
		s.conn.Close()
		s.cancel()
		sl.Debug("WriteLoop exited", "referenceID", s.referenceID)
	}()
	for {
		select {
		case msg, ok := <-s.send:
			if !ok {
				return
			}
			wsutil.WriteServerBinary(s.conn, msg)
		case <-ticker.C:
			sl.Log(context.Background(), slog.Level(-8), "ping",
				slog.Group("player",
					"id", s.referenceID,
				),
			)
			wsutil.WriteServerMessage(s.conn, ws.OpPing, []byte("ping"))
		case <-s.ctx.Done():
			// EXIT AND CLOSE SOCKET SENT FROM ABOVE
			//s.Messages <- s.unregisterMessage()
			return
		}
	}
}

func (s *SocketSession[PlayerId]) unregisterMessage() SocketMessage[PlayerId] {
	return SocketMessage[PlayerId]{
		ReferenceID: s.referenceID,
		Type:        Disconnect,
		Message:     nil,
	}
}

func (s *SocketSession[PlayerId]) Send(message []byte) {
	s.send <- message
}
