package main

import (
	"context"
	"errors"
	"github.com/chilledoj/examples/chat/chat"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
)

func main() {

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	txtHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	})
	slogger := slog.New(txtHandler)

	slog.SetDefault(slogger)

	cr := chat.NewChatRoom(context.Background())
	cr.Start()

	s := http.Server{
		Addr: ":10101",
	}

	http.HandleFunc("/chat", chatHandler(cr))

	http.Handle("/", http.FileServer(http.Dir("./public/")))

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()
	<-ctx.Done()
	slog.Info("shutting down rooms")
	cr.Stop()
	slog.Info("shutting down server")
	s.Shutdown(context.Background())
	slog.Info("shutdown complete")
}

func chatHandler(room *chat.ChatRoom) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		if username == "" {
			http.Error(w, "username is required", http.StatusBadRequest)
			return
		}
		user := room.NewUser(username)

		room.HandleSocketWithPlayer(user.Id, func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "error", http.StatusInternalServerError)
		})(w, r)

	}
}
