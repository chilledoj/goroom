package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/chilledoj/goroom/room"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
)

var lobbyStore sync.Map

type PlayerIdentifier = int64
type RoomIdentifier = int64

type Player struct {
	ID          PlayerIdentifier `json:"id"`
	Name        string           `json:"name"`
	Avatar      string           `json:"avatar"`
	CurrentRoom RoomIdentifier   `json:"currentRoom"`
	IsConnected bool             `json:"isConnected"`
}

type Lobby struct {
	*room.Room[RoomIdentifier, PlayerIdentifier]
	owner  PlayerIdentifier
	status string
}

func (l *Lobby) OnConnect(playerId PlayerIdentifier) {
	slog.Info("player connected", "playerId", playerId)
	object := map[string]interface{}{
		"lobby":            l.ID,
		"event":            "player_update",
		"player":           playerId,
		"allocatedPlayers": l.GetPlayerPresence(),
		"ownerId":          l.owner,
	}
	data, _ := json.Marshal(object)
	payload := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(payload, data)
	l.Room.SendMessageToPlayer(playerId, payload)
}

func NewLobby(parentCtx context.Context, owner Player) *Lobby {
	roomId := rand.Int63()
	lobby := &Lobby{
		owner:  owner.ID,
		status: "open",
	}

	lobby.Room = room.NewRoom[RoomIdentifier, PlayerIdentifier](parentCtx, roomId, room.Options[PlayerIdentifier]{
		OnConnect: lobby.OnConnect,
		OnDisconnect: func(player PlayerIdentifier) {
			fmt.Printf("player disconnected: %d\n", player)
		},
		OnMessage: func(player PlayerIdentifier, message []byte) {
			fmt.Printf("player %d sent message: %s\n", player, message)
		},
	})

	go lobby.Room.Start()

	return lobby
}

func main() {

	slog.SetLogLoggerLevel(slog.LevelDebug)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := http.Server{
		Addr:    ":10101",
		Handler: r,
	}

	r.Mount("/debug", middleware.Profiler())

	r.HandleFunc("/api/players/me", func(w http.ResponseWriter, r *http.Request) {
		player := getPlayerFromHTTP(r)
		jsonResponse(w, player)
	})

	r.HandleFunc("GET /api/lobbies", func(w http.ResponseWriter, r *http.Request) {
		lobbies := make([]lobbyResponse, 0)
		lobbyStore.Range(func(key, value interface{}) bool {
			l, ok := value.(*Lobby)
			if !ok {
				return false
			}

			lobbies = append(lobbies, l.toResponse())
			return true
		})

		jsonResponse(w, lobbies)
	})

	r.HandleFunc("GET /api/lobbies/{lobbyId}", func(w http.ResponseWriter, r *http.Request) {
		lobbyId := chi.URLParam(r, "lobbyId")
		l, ok := lobbyStore.Load(lobbyId)
		if !ok {
			http.Error(w, "lobby not found", http.StatusNotFound)
			return
		}

		lobby, ok := l.(*Lobby)
		if !ok {
			http.Error(w, "lobby is not lobby", http.StatusNotFound)
			return
		}

		jsonResponse(w, lobby.toResponse())
	})

	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r.HandleFunc("POST /api/lobbies", func(w http.ResponseWriter, r *http.Request) {
		player := getPlayerFromHTTP(r)

		lobby := NewLobby(mainCtx, player)

		slog.Info("new lobby created", "lobbyId", lobby.ID)
		lobbyStore.Store(lobby.ID, lobby)
		jsonResponse(w, lobby.toResponse())
	})

	r.HandleFunc("GET /api/lobbies/{lobbyId}/ws", func(w http.ResponseWriter, r *http.Request) {
		player := getPlayerFromHTTP(r)

		lobbyIdStr := chi.URLParam(r, "lobbyId")
		slog.Info("lobby websocket", "lobbyId", lobbyIdStr)

		lobbyId, err := strconv.ParseInt(lobbyIdStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid lobby id", http.StatusBadRequest)
			return
		}

		lobbyStore.Range(func(key, value any) bool {
			slog.Info("lobbyStore", "key", key, "value", value, "requested", key == lobbyId)
			return true
		})

		l, ok := lobbyStore.Load(lobbyId)
		if !ok {
			http.Error(w, "lobby not found", http.StatusNotFound)
			return
		}

		lobby, ok := l.(*Lobby)
		if !ok {
			http.Error(w, "lobby is not lobby", http.StatusNotFound)
			return
		}

		lobby.Room.HandleSocketWithPlayer(player.ID, func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusNotFound)
		})(w, r)
	})

	r.Handle("/*", http.FileServer(http.Dir("./public/")))

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
		}
	}()

	<-mainCtx.Done()

	slog.Info("shutting down")
	s.Shutdown(context.Background())
	slog.Info("shutting down rooms")
	lobbyStore.Range(func(key, value interface{}) bool {
		l, ok := value.(*Lobby)
		if !ok {
			return false
		}
		l.Stop()
		return true
	})
	slog.Info("shutdown complete")

}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	buf, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
}

var onlyPlayer = Player{
	ID:     123456789,
	Name:   "test",
	Avatar: "gopher",
}

func getPlayerFromHTTP(r *http.Request) Player {
	return onlyPlayer
}

type lobbyResponse struct {
	ID      RoomIdentifier   `json:"id"`
	Status  string           `json:"status"`
	OwnerID PlayerIdentifier `json:"ownerId"`
	Players []Player         `json:"players"`
}

func (l *Lobby) toResponse() lobbyResponse {
	roomPlayers := l.GetPlayerPresence()
	players := make([]Player, len(roomPlayers))
	for idx, p := range roomPlayers {
		// normally get user from store
		player := onlyPlayer
		player.IsConnected = p.IsConnected
		player.CurrentRoom = l.ID

		players[idx] = Player{}
	}
	return lobbyResponse{
		ID:      l.ID,
		Status:  l.status,
		OwnerID: l.owner,
		Players: []Player{},
	}
}
