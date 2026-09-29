package worker

import (
	"clipflow/internal/config"
	"clipflow/internal/media"
	"clipflow/internal/model"
	"clipflow/internal/queue"
	"clipflow/internal/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Worker struct {
	Queue            *queue.Queue
	Store            *storage.Store
	ID               string
	Lease, Heartbeat time.Duration
}

func (w *Worker) Run(ctx context.Context) error {
	if w.Heartbeat >= w.Lease {
		return fmt.Errorf("heartbeat must be shorter than lease")
	}
	for ctx.Err() == nil {
		j, err := w.Queue.Claim(ctx, w.ID, w.Lease)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				slog.Error("claim_failed", "worker", w.ID, "error", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}
		w.Handle(ctx, j)
	}
	return ctx.Err()
}
func (w *Worker) Handle(parent context.Context, j model.Job) {
	log := slog.Default().With("job", j.ID, "attempt", j.AttemptCount, "token", j.Token, "worker", w.ID)
	log.Info("claimed")
	ctx, cancel := context.WithTimeout(parent, 120*time.Second)
	defer cancel()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(w.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Only the obsolete-worker demo deliberately abandons renewal.
				// Publication still uses the normal database ownership guard.
				if config.Env("CLIPFLOW_DEMO_MODE", "") == "1" && config.Env("PAUSE_BEFORE_PUBLISH", "") == "1" && config.Env("DEMO_ABANDON_HEARTBEAT", "") == "1" && j.AttemptCount == 1 {
					continue
				}
				hbCtx, hbCancel := context.WithTimeout(ctx, 3*time.Second)
				err := w.Queue.Heartbeat(hbCtx, j, w.Lease)
				hbCancel()
				if err != nil {
					log.Warn("lease_lost", "error", err)
					cancel()
					return
				}
			}
		}
	}()
	err, category, permanent := w.process(ctx, j, log)
	close(done)
	<-stopped
	if err != nil {
		log.Warn("attempt_failed", "category", category, "error", err)
		failCtx, failCancel := context.WithTimeout(parent, 5*time.Second)
		defer failCancel()
		if e := w.Queue.Fail(failCtx, j, category, err.Error(), permanent); e != nil {
			log.Warn("failure_record_rejected", "error", e)
		}
	} else {
		log.Info("published")
	}
}
func (w *Worker) process(ctx context.Context, j model.Job, log *slog.Logger) (error, string, bool) {
	dir, err := os.MkdirTemp("", "clipflow-process-")
	if err != nil {
		return err, "local_io", false
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input.mp4")
	objCtx, cancel := storage.Timeout(ctx)
	err = w.Store.Download(objCtx, j.InputKey, input)
	cancel()
	if err != nil {
		return err, "storage_unavailable", false
	}
	if _, err = media.Probe(ctx, input); err != nil {
		if ctx.Err() != nil {
			return ctx.Err(), "timeout", false
		}
		return err, "invalid_media", true
	}
	m, err := media.Process(ctx, input, dir)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err(), "timeout", false
		}
		return err, "processing_failed", true
	}
	prefix := queue.Prefix(j)
	for _, kind := range []string{"thumbnail", "preview", "metadata"} {
		ext := map[string]string{"thumbnail": ".jpg", "preview": ".mp4", "metadata": ".json"}[kind]
		m.Objects[kind] = prefix + kind + ext
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err, "metadata_invalid", true
	}
	if err = os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0600); err != nil {
		return err, "local_io", false
	}
	for _, kind := range []string{"thumbnail", "preview", "metadata"} {
		name := map[string]string{"thumbnail": "thumbnail.jpg", "preview": "preview.mp4", "metadata": "metadata.json"}[kind]
		ct := map[string]string{"thumbnail": "image/jpeg", "preview": "video/mp4", "metadata": "application/json"}[kind]
		putCtx, putCancel := storage.Timeout(ctx)
		err = w.Store.Put(putCtx, m.Objects[kind], filepath.Join(dir, name), ct)
		putCancel()
		if err != nil {
			return err, "storage_unavailable", false
		}
	}
	// Opt-in file barrier is restricted to explicit test/demo mode. A stale process
	// may outlive its heartbeat; Publish must reject it independently of cancellation.
	if config.Env("CLIPFLOW_DEMO_MODE", "") == "1" && config.Env("PAUSE_BEFORE_PUBLISH", "") == "1" && j.AttemptCount == 1 {
		marker := "/tmp/clipflow-paused-" + j.ID
		if err = os.WriteFile(marker, []byte(j.Token), 0600); err != nil {
			return err, "local_io", false
		}
		defer os.Remove(marker)
		defer os.Remove(marker + ".release")
		log.Info("publication_paused", "marker", marker)
		for {
			if _, e := os.Stat(marker + ".release"); e == nil {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err(), "cancelled", false
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if err = w.Queue.Publish(ctx, j, m); err != nil {
		return err, "stale_publication", false
	}
	return nil, "", false
}
