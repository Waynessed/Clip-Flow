package integration_test

import (
	"bytes"
	"clipflow/internal/cleanup"
	"clipflow/internal/httpapi"
	"clipflow/internal/media"
	"clipflow/internal/model"
	"clipflow/internal/queue"
	"clipflow/internal/service"
	"clipflow/internal/storage"
	"clipflow/internal/worker"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Separate schema/bucket let tests run beside the demo without consuming its jobs.
func setup(t *testing.T) (context.Context, *queue.Queue, *storage.Store) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires TEST_DATABASE_URL and real PostgreSQL/MinIO/FFmpeg")
	}
	ctx := context.Background()
	base, err := pgxpool.New(ctx, url)
	must(t, err)
	schema := "test_" + strings.ReplaceAll(model.UUID(), "-", "")
	_, err = base.Exec(ctx, "CREATE SCHEMA "+schema)
	must(t, err)
	cfg, err := pgxpool.ParseConfig(url)
	must(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	must(t, err)
	q := &queue.Queue{DB: db}
	must(t, q.Migrate(ctx))
	s := storage.New()
	s.Bucket = "test-" + model.UUID()
	must(t, s.Ensure(ctx))
	t.Cleanup(func() {
		objs, _ := s.List(ctx)
		for _, o := range objs {
			if o.Key != nil {
				s.Delete(ctx, *o.Key)
			}
		}
		s.Client.DeleteBucket(ctx, nilSafeBucket(s.Bucket))
		db.Close()
		base.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		base.Close()
	})
	return ctx, q, s
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func clip(t *testing.T, name string, audio bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".mp4")
	args := []string{"-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=960x540:rate=12"}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100")
	}
	args = append(args, "-t", "1", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", path)
	out, err := exec.Command("ffmpeg", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("generate clip: %v %s", err, out)
	}
	return path
}
func upload(t *testing.T, s *service.Service, path, key string) model.Job {
	t.Helper()
	f, err := os.Open(path)
	must(t, err)
	defer f.Close()
	j, err := s.Upload(context.Background(), f, key, "clip.mp4")
	must(t, err)
	return j
}
func insert(t *testing.T, q *queue.Queue, input string) model.Job {
	t.Helper()
	id := model.UUID()
	j, err := q.Insert(context.Background(), id, id, "hash", input, "clip.mp4")
	must(t, err)
	return j
}
func expire(t *testing.T, q *queue.Queue, j model.Job) {
	t.Helper()
	_, err := q.DB.Exec(context.Background(), "UPDATE jobs SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE id=$1", j.ID)
	must(t, err)
}

func TestRealServiceScenarios(t *testing.T) {
	ctx, q, s := setup(t)
	svc := &service.Service{Queue: q, Store: s}
	path := clip(t, "valid", true)
	reset := func() { _, err := q.DB.Exec(ctx, "TRUNCATE job_attempts,jobs"); must(t, err) }
	t.Run("ten_duplicate_and_concurrent_uploads", func(t *testing.T) {
		defer reset()
		key := model.UUID()
		first := upload(t, svc, path, key)
		for i := 0; i < 9; i++ {
			if got := upload(t, svc, path, key); got.ID != first.ID {
				t.Fatal("duplicate durable job")
			}
		}
		var wg sync.WaitGroup
		ids := make(chan string, 10)
		errs := make(chan error, 10)
		key = model.UUID()
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				f, err := os.Open(path)
				if err != nil {
					errs <- err
					return
				}
				defer f.Close()
				j, err := svc.Upload(ctx, f, key, "concurrent.mp4")
				if err != nil {
					errs <- err
					return
				}
				ids <- j.ID
			}()
		}
		wg.Wait()
		close(ids)
		close(errs)
		for err := range errs {
			must(t, err)
		}
		var id string
		for got := range ids {
			if id != "" && id != got {
				t.Fatal("concurrent key created distinct jobs")
			}
			id = got
		}
		var count int
		must(t, q.DB.QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&count))
		if count != 2 {
			t.Fatalf("got %d durable jobs", count)
		}
		other := clip(t, "noaudio", false)
		f, err := os.Open(other)
		must(t, err)
		defer f.Close()
		_, err = svc.Upload(ctx, f, key, "other.mp4")
		if !errors.Is(err, queue.ErrConflict) {
			t.Fatalf("expected conflict, got %v", err)
		}
	})
	t.Run("http_contract_and_bounds", func(t *testing.T) {
		defer reset()
		server := httptest.NewServer((&httpapi.API{Service: svc}).Handler())
		defer server.Close()
		send := func(key string, data []byte, extra bool) int {
			var b bytes.Buffer
			mw := multipart.NewWriter(&b)
			part, _ := mw.CreateFormFile("file", "clip.mp4")
			part.Write(data)
			if extra {
				p, _ := mw.CreateFormField("unexpected")
				p.Write([]byte("extra"))
			}
			mw.Close()
			req, _ := http.NewRequest("POST", server.URL+"/v1/jobs", &b)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			if key != "" {
				req.Header.Set("Idempotency-Key", key)
			}
			r, err := http.DefaultClient.Do(req)
			must(t, err)
			defer r.Body.Close()
			return r.StatusCode
		}
		data, err := os.ReadFile(path)
		must(t, err)
		key := model.UUID()
		if send("", data, false) != 400 {
			t.Fatal("missing key")
		}
		if send(key, data, true) != 400 {
			t.Fatal("extra part")
		}
		if send(key, []byte("corrupt"), false) != 422 {
			t.Fatal("corrupt accepted")
		}
		if send(key, make([]byte, service.MaxUpload+1), false) != 413 {
			t.Fatal("oversize accepted")
		}
		if send(key, data, false) != 202 {
			t.Fatal("valid rejected")
		}
		other, err := os.ReadFile(clip(t, "different", false))
		must(t, err)
		if send(key, other, false) != 409 {
			t.Fatal("different input did not conflict")
		}
	})
	t.Run("concurrent_claims_and_stale_publication", func(t *testing.T) {
		defer reset()
		job := insert(t, q, "missing")
		var wg sync.WaitGroup
		claims := make(chan model.Job, 8)
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				j, err := q.Claim(ctx, model.UUID(), 60*time.Second)
				if err == nil {
					claims <- j
				} else if !errors.Is(err, pgx.ErrNoRows) {
					errs <- err
				}
			}()
		}
		wg.Wait()
		close(claims)
		close(errs)
		for err := range errs {
			must(t, err)
		}
		var old model.Job
		count := 0
		for j := range claims {
			old = j
			count++
		}
		if count != 1 || old.ID != job.ID {
			t.Fatalf("%d active claims", count)
		}
		expire(t, q, old)
		if err := q.Heartbeat(ctx, old, time.Minute); !errors.Is(err, queue.ErrStale) {
			t.Fatal("expired heartbeat accepted")
		}
		if err := q.Publish(ctx, old, model.Manifest{}); !errors.Is(err, queue.ErrStale) {
			t.Fatal("expired publication accepted")
		}
		current, err := q.Claim(ctx, "recovery", time.Minute)
		must(t, err)
		if current.Token == old.Token || current.AttemptCount != 2 {
			t.Fatal("no new attempt")
		}
		if err = q.Publish(ctx, old, model.Manifest{}); !errors.Is(err, queue.ErrStale) {
			t.Fatal("obsolete publication accepted")
		}
		must(t, q.Publish(ctx, current, model.Manifest{Objects: map[string]string{"preview": "current"}}))
		got, err := q.Get(ctx, job.ID)
		must(t, err)
		if got.Attempts[0].Outcome != "expired" || got.Attempts[1].Outcome != "succeeded" {
			t.Fatal("incorrect attempt history")
		}
	})
	t.Run("retry_delays_exhaustion_and_corrupt_worker_input", func(t *testing.T) {
		defer reset()
		job := insert(t, q, "missing-object")
		w := worker.Worker{Queue: q, Store: s, ID: "test", Lease: time.Minute, Heartbeat: 10 * time.Second}
		for n := 1; n <= 3; n++ {
			j, err := q.Claim(ctx, "test", time.Minute)
			must(t, err)
			w.Handle(ctx, j)
			got, err := q.Get(ctx, job.ID)
			must(t, err)
			if got.AttemptCount != n {
				t.Fatal("attempt count")
			}
			if n < 3 {
				var delay float64
				must(t, q.DB.QueryRow(ctx, "SELECT extract(epoch FROM available_at-updated_at) FROM jobs WHERE id=$1", job.ID).Scan(&delay))
				want := 5.0
				if n == 2 {
					want = 15
				}
				if delay < want-.1 || delay > want+.1 {
					t.Fatalf("retry delay %f expected %f", delay, want)
				}
				_, err = q.DB.Exec(ctx, "UPDATE jobs SET available_at=clock_timestamp() WHERE id=$1", job.ID)
				must(t, err)
			} else if got.State != "failed" {
				t.Fatal("not exhausted")
			}
		}
		bad := filepath.Join(t.TempDir(), "corrupt.mp4")
		must(t, os.WriteFile(bad, []byte("corrupt"), 0600))
		must(t, s.Put(ctx, "corrupt", bad, "video/mp4"))
		insert(t, q, "corrupt")
		j, err := q.Claim(ctx, "test", time.Minute)
		must(t, err)
		w.Handle(ctx, j)
		got, err := q.Get(ctx, j.ID)
		must(t, err)
		if got.State != "failed" || got.AttemptCount != 1 || *got.ErrorCategory != "invalid_media" {
			t.Fatal("corrupt input was not permanent")
		}
	})
	t.Run("expired_attempts_count_toward_maximum", func(t *testing.T) {
		defer reset()
		job := insert(t, q, "missing")
		for i := 0; i < 3; i++ {
			j, err := q.Claim(ctx, "dead", time.Second)
			must(t, err)
			expire(t, q, j)
		}
		_, err := q.Claim(ctx, "fourth", time.Minute)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("fourth claim %v", err)
		}
		got, err := q.Get(ctx, job.ID)
		must(t, err)
		if got.State != "failed" || len(got.Attempts) != 3 {
			t.Fatal("expiry exhaustion failed")
		}
	})
	t.Run("heartbeat_renews_then_cancels_lost_ownership", func(t *testing.T) {
		defer reset()
		t.Setenv("CLIPFLOW_DEMO_MODE", "1")
		t.Setenv("PAUSE_BEFORE_PUBLISH", "1")
		t.Setenv("DEMO_ABANDON_HEARTBEAT", "")
		job := upload(t, svc, path, model.UUID())
		j, err := q.Claim(ctx, "heartbeat-test", 2*time.Second)
		must(t, err)
		w := &worker.Worker{Queue: q, Store: s, ID: "heartbeat-test", Lease: 2 * time.Second, Heartbeat: 100 * time.Millisecond}
		done := make(chan struct{})
		go func() { w.Handle(ctx, j); close(done) }()
		marker := "/tmp/clipflow-paused-" + job.ID
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(marker); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("publication barrier not reached")
			}
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(2200 * time.Millisecond)
		var valid bool
		must(t, q.DB.QueryRow(ctx, "SELECT lease_expires_at>clock_timestamp() FROM jobs WHERE id=$1", j.ID).Scan(&valid))
		if !valid {
			t.Fatal("heartbeat did not renew lease past original expiry")
		}
		expire(t, q, j)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("lost ownership did not cancel processing")
		}
		current, err := q.Claim(ctx, "recovery", time.Minute)
		must(t, err)
		w.Lease = time.Minute
		w.Heartbeat = time.Second
		w.Handle(ctx, current)
		got, err := q.Get(ctx, j.ID)
		must(t, err)
		if got.State != "succeeded" || got.Attempts[0].Outcome != "expired" {
			t.Fatal("heartbeat recovery failed")
		}
	})
	t.Run("real_outputs_metadata_and_restart", func(t *testing.T) {
		defer reset()
		job := upload(t, svc, path, model.UUID())
		url := os.Getenv("TEST_DATABASE_URL")
		cfg, err := pgxpool.ParseConfig(url)
		must(t, err)
		var schema string
		must(t, q.DB.QueryRow(ctx, "SELECT current_schema()").Scan(&schema))
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
		newPool, err := pgxpool.NewWithConfig(ctx, cfg)
		must(t, err)
		defer newPool.Close()
		restarted := &queue.Queue{DB: newPool}
		j, err := restarted.Claim(ctx, "restarted", time.Minute)
		must(t, err)
		workerInstance := &worker.Worker{Queue: restarted, Store: s, ID: "restarted", Lease: time.Minute, Heartbeat: 10 * time.Second}
		workerInstance.Handle(ctx, j)
		got, err := q.Get(ctx, job.ID)
		must(t, err)
		if got.State != "succeeded" {
			t.Fatalf("%+v", got)
		}
		out := filepath.Join(t.TempDir(), "preview.mp4")
		must(t, s.Download(ctx, got.Manifest.Objects["preview"], out))
		pm, err := media.Probe(ctx, out)
		must(t, err)
		if pm != got.Manifest.Preview {
			t.Fatal("metadata does not match real preview")
		}
		if pm.Height > 480 || !pm.Audio {
			t.Fatal("bad preview")
		}
		obj, err := s.Get(ctx, got.Manifest.Objects["metadata"])
		must(t, err)
		raw, err := io.ReadAll(obj.Body)
		obj.Body.Close()
		must(t, err)
		var m model.Manifest
		must(t, json.Unmarshal(raw, &m))
		if m.Preview != pm {
			t.Fatal("metadata object differs")
		}
		for _, key := range []string{job.InputKey, got.Manifest.Objects["preview"], got.Manifest.Objects["thumbnail"], got.Manifest.Objects["metadata"]} {
			yes, err := q.Referenced(ctx, key)
			must(t, err)
			if !yes {
				t.Fatalf("cleanup would remove referenced %s", key)
			}
		}
		yes, err := q.Referenced(ctx, "attempts/unreferenced/old/preview.mp4")
		must(t, err)
		if yes {
			t.Fatal("orphan marked referenced")
		}
		orphan := "attempts/unreferenced/old/preview.mp4"
		must(t, s.Put(ctx, orphan, path, "video/mp4"))
		insert(t, q, "inputs/active")
		active, err := q.Claim(ctx, "cleanup-active", time.Minute)
		must(t, err)
		activeKey := queue.Prefix(active) + "preview.mp4"
		must(t, s.Put(ctx, activeKey, path, "video/mp4"))
		// Test-only future cutoff makes newly uploaded fixtures eligible. Unit tests
		// independently prove the CLI's 24-hour age rule; production uses real time.
		deleted, err := cleanup.Sweep(ctx, q, s, time.Now().Add(time.Hour))
		must(t, err)
		found := false
		for _, key := range deleted {
			if key == orphan {
				found = true
			}
		}
		if !found {
			t.Fatal("real orphan was not deleted")
		}
		for _, key := range []string{job.InputKey, got.Manifest.Objects["preview"], got.Manifest.Objects["thumbnail"], got.Manifest.Objects["metadata"], activeKey} {
			obj, err := s.Get(ctx, key)
			must(t, err)
			obj.Body.Close()
		}
		metrics, err := q.Metrics(ctx)
		must(t, err)
		if !strings.Contains(metrics, "clipflow_processing_seconds_count") {
			t.Fatal("missing duration metrics")
		}
	})
}
