package main

import (
	"clipflow/internal/config"
	"clipflow/internal/httpapi"
	"clipflow/internal/queue"
	"clipflow/internal/service"
	"clipflow/internal/storage"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	if err = q.Migrate(ctx); err != nil {
		slog.Error("migration", "error", err)
		os.Exit(1)
	}
	s := storage.New()
	readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = s.Ensure(readyCtx)
	cancel()
	if err != nil {
		slog.Error("storage_startup", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: ":8080", Handler: (&httpapi.API{Service: &service.Service{Queue: q, Store: s}}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 45 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(c)
	}()
	slog.Info("listening", "port", 8080)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server", "error", err)
		os.Exit(1)
	}
}
