package main

import (
	"clipflow/internal/cleanup"
	"clipflow/internal/config"
	"clipflow/internal/queue"
	"clipflow/internal/storage"
	"context"
	"log/slog"
	"os"
	"time"
)

func main() {
	config.Logging()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	q, err := queue.Open(ctx, config.Env("DATABASE_URL", ""))
	if err != nil {
		slog.Error("cleanup_refused", "error", err)
		os.Exit(1)
	}
	defer q.DB.Close()
	deleted, err := cleanup.Sweep(ctx, q, storage.New(), time.Now().Add(-24*time.Hour))
	for _, key := range deleted {
		slog.Info("deleted", "key", key)
	}
	if err != nil {
		slog.Error("cleanup_refused", "error", err)
		os.Exit(1)
	}
	slog.Info("cleanup_complete", "deleted", len(deleted))
}
