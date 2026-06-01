package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/httpapi"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
)

func main() {
	cfg := config.Load()
	log := config.Logger()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	storageEngine, err := store.NewStorageEngine(startupCtx, cfg.DatabaseURL)
	cancelStartup()
	if err != nil {
		log.Error("storage engine init failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer storageEngine.Close()

	hub := realtime.NewHub(storageEngine.Repository(), log)
	api := httpapi.NewServer(cfg, hub, log, storageEngine)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		log.Info("backend listening", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Info("shutdown requested")
	case err := <-errs:
		log.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpapi.Shutdown(shutdownCtx, server); err != nil {
		log.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("backend stopped")
}
