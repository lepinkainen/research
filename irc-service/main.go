package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lepinkainen/research/irc-service/db"
	"github.com/lepinkainen/research/irc-service/irc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := loadConfig()

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open db", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}
	defer store.Close()
	slog.Info("db ready", "path", cfg.DBPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr := irc.NewManager(store)
	if nets := seedNetworksFromEnv(); len(nets) > 0 {
		if err := mgr.Start(ctx, nets); err != nil {
			slog.Error("start irc manager", "err", err)
			os.Exit(1)
		}
		slog.Info("irc networks started", "count", len(nets))
	} else {
		slog.Info("no networks configured; set IRC_NETWORK to connect one")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.PingContext(r.Context()); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("http listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http serve", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "err", err)
	}
	mgr.Wait()
	slog.Info("bye")
}
