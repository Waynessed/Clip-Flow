package service

import (
	"clipflow/internal/media"
	"clipflow/internal/model"
	"clipflow/internal/queue"
	"clipflow/internal/storage"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"os"
)

const MaxUpload = 20 * 1024 * 1024

var ErrTooLarge = errors.New("file exceeds 20 MB")

type InvalidMedia struct{ Cause error }

func (e InvalidMedia) Error() string { return e.Cause.Error() }

type Service struct {
	Queue *queue.Queue
	Store *storage.Store
}

func (s *Service) Upload(ctx context.Context, r io.Reader, key, filename string) (model.Job, error) {
	f, err := os.CreateTemp("", "clipflow-upload-*.mp4")
	if err != nil {
		return model.Job{}, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, MaxUpload+1))
	if err != nil {
		return model.Job{}, err
	}
	if n > MaxUpload {
		return model.Job{}, ErrTooLarge
	}
	if err = f.Close(); err != nil {
		return model.Job{}, err
	}
	if _, err = media.Probe(ctx, f.Name()); err != nil {
		return model.Job{}, InvalidMedia{err}
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	j, err := s.Queue.ByKey(ctx, key, digest)
	if err == nil || errors.Is(err, queue.ErrConflict) {
		return j, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return j, err
	}
	id := model.UUID()
	object := "inputs/" + id + ".mp4"
	objCtx, cancel := storage.Timeout(ctx)
	defer cancel()
	if err = s.Store.Put(objCtx, object, f.Name(), "video/mp4"); err != nil {
		return j, fmt.Errorf("store input: %w", err)
	}
	return s.Queue.Insert(ctx, id, key, digest, object, filename)
}
