package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/go-common/pkg/config"
	lobby "github.com/QuizWars-Ecosystem/lobby-service/internal/config"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/server"
)

func main() {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "app/config/config.yaml"
	}

	manager, err := config.NewManager[lobby.Config](path)
	if err != nil {
		slog.Error("Error loading config: ", "error", err)
		return
	}

	cfg := manager.Config()

	startCtx, startCancel := context.WithTimeout(context.Background(), cfg.StartTimeout)
	defer startCancel()

	var srv abstractions.Server

	srv, err = server.NewServer(startCtx, manager)
	if err != nil {
		slog.Error("Error starting server: ", "error", err)
		return
	}

	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		<-signalCh
		slog.Info("Shutting down server...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.StopTimeout)
		defer shutdownCancel()

		if err = srv.Shutdown(shutdownCtx); err != nil {
			slog.Warn("Server forced to shutdown", "error", err)
		}
	}()

	if err = srv.Start(); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}
