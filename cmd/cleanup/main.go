package main

import (
	"clipflow/internal/config"
	"clipflow/internal/queue"
	"clipflow/internal/storage"
	"context"
	"log/slog"
	"os"
	"strings"
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
	s := storage.New()
	objects, err := s.List(ctx)
	if err != nil {
		slog.Error("cleanup_refused", "error", err)
		os.Exit(1)
	}
	for _, o := range objects {
		if o.Key == nil || o.LastModified == nil || time.Since(*o.LastModified) < 24*time.Hour {
			continue
		}
		key := *o.Key
		if !strings.HasPrefix(key, "inputs/") && !strings.HasPrefix(key, "attempts/") {
			continue
		}
		yes, err := q.Referenced(ctx, key)
		if err != nil {
			slog.Error("cleanup_refused", "error", err)
			os.Exit(1)
		}
		if yes {
			continue
		}
		if err = s.Delete(ctx, key); err != nil {
			slog.Error("delete_failed", "key", key, "error", err)
			os.Exit(1)
		}
		slog.Info("deleted", "key", key)
	}
}
