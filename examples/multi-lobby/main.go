package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/chilledoj/goroom"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringLength(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Int63()%int64(len(letters))]
	}
	return string(b)
}

var playerStore sync.Map
var lobbyStore sync.Map

type PlayerIdentifier = int8
type RoomIdentifier = string

type Player struct {
	ID          PlayerIdentifier `json:"id"`
	Username    string           `json:"username"`
	Avatar      string           `json:"avatar"`
	CurrentRoom RoomIdentifier   `json:"currentRoom"`
	IsConnected bool             `json:"isConnected"`
}

type Lobby struct {
	*goroom.Room[RoomIdentifier, PlayerIdentifier]
	owner            PlayerIdentifier
	allocatedPlayers []PlayerIdentifier
}

func (l *Lobby) OnConnect(playerId PlayerIdentifier) {
	slog.Info("player connected", "playerId", playerId)
	found := false
	for _, p := range l.allocatedPlayers {
		if p == playerId {
			found = true
			break
		}
	}
	if !found {
		l.allocatedPlayers = append(l.allocatedPlayers, playerId)
	}
	data, _ := json.Marshal(l.toResponse())
	//payload := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	//base64.StdEncoding.Encode(payload, data)
	l.Room.SendMessageToPlayer(playerId, data)
	l.Room.SendMessageToAllPlayers(data)
}

func (l *Lobby) OnDisconnect(playerId PlayerIdentifier) {
	slog.Info("player disconnected", "playerId", playerId)
	for idx, p := range l.allocatedPlayers {
		if p == playerId {
			l.allocatedPlayers = append(l.allocatedPlayers[:idx], l.allocatedPlayers[idx+1:]...)
			break
		}
	}
	data, _ := json.Marshal(l.toResponse())
	l.Room.SendMessageToAllPlayers(data)
}

type PlayerMessage struct {
	Action   string           `json:"action"`
	LobbyId  string           `json:"lobbyId"`
	PlayerId PlayerIdentifier `json:"playerId"`
	Status   string           `json:"status"`
}

func (l *Lobby) ProcessMessage(playerId PlayerIdentifier, message []byte) {
	slog.Info("player message", "playerId", playerId, "message", string(message))
	var msg PlayerMessage
	err := json.Unmarshal(message, &msg)
	if err != nil {
		slog.Error("unmarshal", "err", err)
		return
	}
	switch msg.Action {
	case "toggleStatus":
		l.changeRoomStatus(msg.Status)
	}
}

func (l *Lobby) changeRoomStatus(newStatus string) {
	var roomStatus goroom.RoomStatus
	switch newStatus {
	case "Open":
		roomStatus = goroom.Open
	case "Locked":
		roomStatus = goroom.Locked
	default:
		return
	}
	l.Room.SetStatus(roomStatus)
	slog.Info("room status changed", "lobbyId", l.ID, "status", roomStatus.String(), "ownerId", l.owner, "allocatedPlayers", l.allocatedPlayers, "players", l.GetPlayerPresence())

	data, _ := json.Marshal(l.toResponse())
	l.Slogger.Info("sending lobby update", "lobbyId", l.ID, "data", string(data))
	l.Room.SendMessageToAllPlayers(data)
}

func NewLobby(parentCtx context.Context, owner Player) *Lobby {

	roomId := RandStringLength(6)
	lobby := &Lobby{
		owner:            owner.ID,
		allocatedPlayers: []PlayerIdentifier{owner.ID},
	}

	lobby.Room = goroom.NewRoom[RoomIdentifier, PlayerIdentifier](parentCtx, roomId, goroom.Options[PlayerIdentifier]{
		OnConnect:     lobby.OnConnect,
		OnDisconnect:  lobby.OnDisconnect,
		OnMessage:     lobby.ProcessMessage,
		OnRemove:      lobby.OnDisconnect,
		CleanupPeriod: time.Second * 10,
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

	r.HandleFunc("/api/players", func(w http.ResponseWriter, r *http.Request) {})

	r.HandleFunc("POST /api/players", func(w http.ResponseWriter, r *http.Request) {
		postData := struct {
			Username string `json:"username"`
		}{}
		if err := json.NewDecoder(r.Body).Decode(&postData); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		slog.Debug("PlayerInfo", "postData", postData)

		if postData.Username == "" {
			http.Error(w, "username is required", http.StatusBadRequest)
			return
		}

		var foundId PlayerIdentifier
		playerStore.Range(func(key, value interface{}) bool {
			p, ok := value.(Player)
			if !ok {
				return false
			}
			if p.Username == postData.Username {
				foundId = p.ID
				return false
			}
			return true
		})

		var player Player

		if foundId != 0 {
			plyerObj, ok := playerStore.Load(foundId)
			if ok {
				player = plyerObj.(Player)
			}
		}

		if player.ID == 0 {
			pid := rand.Int31n(math.MaxInt8)
			player = Player{
				ID:       PlayerIdentifier(pid),
				Username: postData.Username,
			}
			playerStore.Store(player.ID, player)
		}

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
		player, err := getPlayerFromHTTP(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if player.CurrentRoom != "" {
			http.Error(w, "already in a lobby", http.StatusBadRequest)
			return
		}
		if player.ID == 0 {
		}

		lobby := NewLobby(mainCtx, *player)

		slog.Info("new lobby created", "lobbyId", lobby.ID)
		lobbyStore.Store(lobby.ID, lobby)
		jsonResponse(w, lobby.toResponse())
	})

	r.HandleFunc("GET /api/lobbies/{lobbyId}/ws", func(w http.ResponseWriter, r *http.Request) {
		player, err := getPlayerFromHTTP(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		lobbyIdStr := chi.URLParam(r, "lobbyId")
		slog.Info("lobby websocket", "lobbyId", lobbyIdStr, "playerId", player.ID, "playerUsername", player.Username, "playerAvatar", player.Avatar, "playerCurrentRoom", player.CurrentRoom, "playerIsConnected", player.IsConnected)

		l, ok := lobbyStore.Load(lobbyIdStr)
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
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

// can't be bothered with sessions so will just get the player from the header
func getPlayerFromHTTP(r *http.Request) (*Player, error) {
	qry := r.URL.Query()
	playerIdString := qry["playerId"][0]

	playerId, err := strconv.ParseInt(playerIdString, 10, 8)
	if err != nil {
		return nil, errors.New("invalid player id")
	}
	pid := PlayerIdentifier(playerId)
	playerStore.Range(func(key, value interface{}) bool {
		pid, ok := key.(PlayerIdentifier)
		if !ok {
			return false
		}

		player := value.(Player)
		slog.Info("getPlayerFromHTTP", "key", key, "pid", pid, "checkEqual", player.ID == pid)
		if player.ID != pid {
			return false
		}
		return true
	})
	p, ok := playerStore.Load(pid)
	if !ok {
		return nil, errors.New("player not found")
	}
	player, ok := p.(Player)
	if !ok {
		return nil, errors.New("player is not player")
	}
	return &player, nil
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
		pl, ok := playerStore.Load(p.ID)
		if !ok {
			continue
		}
		player := pl.(Player)
		player.IsConnected = p.IsConnected
		player.CurrentRoom = l.ID

		players[idx] = player
	}
	return lobbyResponse{
		ID:      l.ID,
		Status:  l.Room.Status.String(),
		OwnerID: l.owner,
		Players: players,
	}
}
