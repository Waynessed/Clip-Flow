package httpapi

import (
	"clipflow/internal/queue"
	"clipflow/internal/service"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"time"
)

type API struct{ Service *service.Service }

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	jsonResponse(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := a.Service.Queue.DB.Ping(ctx); err != nil {
			fail(w, 503, "database_unavailable", "PostgreSQL is unavailable")
			return
		}
		if err := a.Service.Store.Ready(ctx); err != nil {
			fail(w, 503, "storage_unavailable", "Object storage is unavailable")
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /v1/jobs", a.upload)
	mux.HandleFunc("GET /v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		j, err := a.Service.Queue.List(r.Context())
		if err != nil {
			fail(w, 503, "database_unavailable", "Could not list jobs")
			return
		}
		jsonResponse(w, 200, j)
	})
	mux.HandleFunc("GET /v1/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !uuid.MatchString(r.PathValue("id")) {
			fail(w, 400, "invalid_id", "Expected a UUID")
			return
		}
		j, err := a.Service.Queue.Get(r.Context(), r.PathValue("id"))
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "not_found", "Job not found")
			return
		}
		if err != nil {
			fail(w, 503, "database_unavailable", "Could not read job")
			return
		}
		jsonResponse(w, 200, j)
	})
	mux.HandleFunc("GET /v1/jobs/{id}/outputs/{kind}", a.output)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		m, err := a.Service.Queue.Metrics(r.Context())
		if err != nil {
			fail(w, 503, "database_unavailable", "Metrics unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		io.WriteString(w, m)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "not_found", "Route not found") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mux.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 200 {
		fail(w, 400, "invalid_key", "Idempotency-Key is required (maximum 200 characters)")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxUpload+64*1024)
	reader, err := r.MultipartReader()
	if err != nil {
		fail(w, 400, "invalid_multipart", "Expected multipart form with one file")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		fail(w, 400, "missing_file", "Expected one file field named file")
		return
	}
	// Validate multipart cardinality before committing any durable job.
	f, err := osTemp(part)
	if err != nil {
		var max *http.MaxBytesError
		if errors.Is(err, service.ErrTooLarge) || errors.As(err, &max) {
			fail(w, 413, "too_large", "File exceeds 20 MB")
		} else {
			fail(w, 400, "invalid_upload", "Could not read upload")
		}
		return
	}
	defer f.Close()
	defer removeTemp(f)
	if _, err = reader.NextPart(); err != io.EOF {
		fail(w, 400, "extra_part", "Upload exactly one file field")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	j, err := a.Service.Upload(ctx, f, key, filepath.Base(part.FileName()))
	if errors.Is(err, queue.ErrConflict) {
		fail(w, 409, "idempotency_conflict", err.Error())
		return
	}
	var invalid service.InvalidMedia
	if errors.As(err, &invalid) {
		fail(w, 422, "invalid_media", invalid.Error())
		return
	}
	if errors.Is(err, service.ErrTooLarge) {
		fail(w, 413, "too_large", err.Error())
		return
	}
	if err != nil {
		slog.Error("upload_failed", "error", err)
		fail(w, 503, "upload_unavailable", "Upload failed; retry with the same idempotency key")
		return
	}
	jsonResponse(w, 202, j)
}
func (a *API) output(w http.ResponseWriter, r *http.Request) {
	id, kind := r.PathValue("id"), r.PathValue("kind")
	if !uuid.MatchString(id) {
		fail(w, 400, "invalid_id", "Expected a UUID")
		return
	}
	if kind != "thumbnail" && kind != "preview" && kind != "metadata" {
		fail(w, 404, "not_found", "Unknown output kind")
		return
	}
	j, err := a.Service.Queue.Get(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "not_found", "Job not found")
		return
	}
	if err != nil {
		fail(w, 503, "database_unavailable", "Could not read job")
		return
	}
	if j.State != "succeeded" || j.Manifest == nil {
		fail(w, 409, "not_ready", "Outputs have not been published")
		return
	}
	key := j.Manifest.Objects[kind]
	if key == "" {
		fail(w, 404, "not_found", "Output not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	obj, err := a.Service.Store.Get(ctx, key)
	if err != nil {
		fail(w, 503, "storage_unavailable", "Output storage unavailable")
		return
	}
	defer obj.Body.Close()
	types := map[string]string{"thumbnail": "image/jpeg", "preview": "video/mp4", "metadata": "application/json"}
	w.Header().Set("Content-Type", types[kind])
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, obj.Body)
}
