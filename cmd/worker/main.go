package main

import (
	"clipflow/internal/config"
	"clipflow/internal/model"
	"clipflow/internal/queue"
	"clipflow/internal/storage"
	"clipflow/internal/worker"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config.Logging()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	q, err := queue.Open(ctx, config.Env("DATABASE_URL", ""))
	if err != nil {
		slog.Error("database_startup", "error", err)
		os.Exit(1)
	}
	defer q.DB.Close()
	hostname, _ := os.Hostname()
	w := worker.Worker{Queue: q, Store: storage.New(), ID: hostname + "-" + model.UUID(), Lease: config.Seconds("LEASE_SECONDS", 60), Heartbeat: config.Seconds("HEARTBEAT_SECONDS", 10)}
	slog.Info("worker_started", "worker", w.ID)
	if err = w.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("worker", "error", err)
		os.Exit(1)
	}
}
