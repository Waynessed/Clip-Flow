package queue

import (
	"clipflow/internal/model"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

//go:embed schema.sql
var schema string
var ErrConflict = errors.New("idempotency key already used with different content")
var ErrStale = errors.New("attempt no longer owns an unexpired lease")

type Queue struct{ DB *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Queue, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Queue{p}, nil
}
func (q *Queue) Migrate(ctx context.Context) error {
	tx, err := q.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(827361)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const columns = `id::text,filename,state,attempt_count,created_at,updated_at,completed_at,error_category,error_message,manifest,input_key,input_hash,COALESCE(lease_token::text,'')`

func scan(row pgx.Row) (model.Job, error) {
	var j model.Job
	var raw []byte
	err := row.Scan(&j.ID, &j.Filename, &j.State, &j.AttemptCount, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt, &j.ErrorCategory, &j.ErrorMessage, &raw, &j.InputKey, &j.InputHash, &j.Token)
	if err != nil {
		return j, err
	}
	j.Attempts = []model.Attempt{}
	if raw != nil {
		if err = json.Unmarshal(raw, &j.Manifest); err != nil {
			return j, err
		}
		j.Outputs = map[string]string{}
		for k := range j.Manifest.Objects {
			j.Outputs[k] = "/v1/jobs/" + j.ID + "/outputs/" + k
		}
	}
	return j, nil
}
func (q *Queue) ByKey(ctx context.Context, key, hash string) (model.Job, error) {
	j, err := scan(q.DB.QueryRow(ctx, "SELECT "+columns+" FROM jobs WHERE idempotency_key=$1", key))
	if err == nil && j.InputHash != hash {
		return j, ErrConflict
	}
	return j, err
}
func (q *Queue) Insert(ctx context.Context, id, key, hash, input, filename string) (model.Job, error) {
	_, err := q.DB.Exec(ctx, `INSERT INTO jobs(id,idempotency_key,input_hash,input_key,filename) VALUES($1,$2,$3,$4,$5) ON CONFLICT(idempotency_key) DO NOTHING`, id, key, hash, input, filename)
	if err != nil {
		return model.Job{}, err
	}
	return q.ByKey(ctx, key, hash)
}
func (q *Queue) Get(ctx context.Context, id string) (model.Job, error) {
	j, err := scan(q.DB.QueryRow(ctx, "SELECT "+columns+" FROM jobs WHERE id=$1", id))
	if err != nil {
		return j, err
	}
	rows, err := q.DB.Query(ctx, `SELECT number,token::text,worker,started_at,ended_at,outcome,error_category,error_message,output_prefix FROM job_attempts WHERE job_id=$1 ORDER BY number`, id)
	if err != nil {
		return j, err
	}
	defer rows.Close()
	for rows.Next() {
		var a model.Attempt
		if err = rows.Scan(&a.Number, &a.Token, &a.Worker, &a.StartedAt, &a.EndedAt, &a.Outcome, &a.ErrorCategory, &a.ErrorMessage, &a.Prefix); err != nil {
			return j, err
		}
		j.Attempts = append(j.Attempts, a)
	}
	return j, rows.Err()
}
func (q *Queue) List(ctx context.Context) ([]model.Job, error) {
	rows, err := q.DB.Query(ctx, "SELECT "+columns+" FROM jobs ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Job{}
	for rows.Next() {
		j, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// Claim holds a row lock only while expiring an old attempt and recording a new one.
func (q *Queue) Claim(ctx context.Context, worker string, lease time.Duration) (model.Job, error) {
	tx, err := q.DB.Begin(ctx)
	if err != nil {
		return model.Job{}, err
	}
	defer tx.Rollback(ctx)
	j, err := scan(tx.QueryRow(ctx, "SELECT "+columns+` FROM jobs WHERE (state='queued' AND available_at<=clock_timestamp()) OR (state='running' AND lease_expires_at<=clock_timestamp()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err != nil {
		return j, err
	}
	if j.State == "running" {
		if _, err = tx.Exec(ctx, `UPDATE job_attempts SET outcome='expired',ended_at=clock_timestamp(),error_category='lease_expired',error_message='Worker lease expired before publication' WHERE token=$1`, j.Token); err != nil {
			return j, err
		}
	}
	if j.AttemptCount >= 3 {
		_, err = tx.Exec(ctx, `UPDATE jobs SET state='failed',completed_at=clock_timestamp(),updated_at=clock_timestamp(),lease_token=NULL,lease_owner=NULL,lease_expires_at=NULL,error_category='retry_exhausted',error_message='Maximum three attempts reached' WHERE id=$1`, j.ID)
		if err != nil {
			return j, err
		}
		if err = tx.Commit(ctx); err != nil {
			return j, err
		}
		return model.Job{}, pgx.ErrNoRows
	}
	j.Token = model.UUID()
	j.AttemptCount++
	j.State = "running"
	_, err = tx.Exec(ctx, `UPDATE jobs SET state='running',attempt_count=$2,lease_owner=$3,lease_token=$4,lease_expires_at=clock_timestamp()+$5*interval '1 second',updated_at=clock_timestamp(),error_category=NULL,error_message=NULL WHERE id=$1`, j.ID, j.AttemptCount, worker, j.Token, lease.Seconds())
	if err != nil {
		return j, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_attempts(job_id,number,token,worker,output_prefix) VALUES($1,$2,$3,$4,$5)`, j.ID, j.AttemptCount, j.Token, worker, Prefix(j))
	if err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}
func Prefix(j model.Job) string { return "attempts/" + j.ID + "/" + j.Token + "/" }
func (q *Queue) Heartbeat(ctx context.Context, j model.Job, lease time.Duration) error {
	r, err := q.DB.Exec(ctx, `UPDATE jobs SET lease_expires_at=clock_timestamp()+$3*interval '1 second' WHERE id=$1 AND lease_token=$2 AND state='running' AND lease_expires_at>clock_timestamp()`, j.ID, j.Token, lease.Seconds())
	if err != nil {
		return err
	}
	if r.RowsAffected() != 1 {
		return ErrStale
	}
	return nil
}
func (q *Queue) Publish(ctx context.Context, j model.Job, m model.Manifest) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tx, err := q.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	r, err := tx.Exec(ctx, `UPDATE jobs SET state='succeeded',manifest=$3,completed_at=clock_timestamp(),updated_at=clock_timestamp(),lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL WHERE id=$1 AND lease_token=$2 AND state='running' AND lease_expires_at>clock_timestamp()`, j.ID, j.Token, raw)
	if err != nil {
		return err
	}
	if r.RowsAffected() != 1 {
		return ErrStale
	}
	if _, err = tx.Exec(ctx, `UPDATE job_attempts SET outcome='succeeded',ended_at=clock_timestamp() WHERE token=$1`, j.Token); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (q *Queue) Fail(ctx context.Context, j model.Job, category, message string, permanent bool) error {
	tx, err := q.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	state := "queued"
	delay := 5
	if j.AttemptCount >= 2 {
		delay = 15
	}
	if permanent || j.AttemptCount >= 3 {
		state = "failed"
	}
	r, err := tx.Exec(ctx, `UPDATE jobs SET state=$3,available_at=clock_timestamp()+$4*interval '1 second',updated_at=clock_timestamp(),completed_at=CASE WHEN $3='failed' THEN clock_timestamp() ELSE NULL END,error_category=$5,error_message=$6,lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL WHERE id=$1 AND lease_token=$2 AND state='running' AND lease_expires_at>clock_timestamp()`, j.ID, j.Token, state, delay, category, message)
	if err != nil {
		return err
	}
	if r.RowsAffected() != 1 {
		return ErrStale
	}
	_, err = tx.Exec(ctx, `UPDATE job_attempts SET outcome=$2,ended_at=clock_timestamp(),error_category=$3,error_message=$4 WHERE token=$1`, j.Token, state, category, message)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (q *Queue) Referenced(ctx context.Context, key string) (bool, error) {
	var yes bool
	err := q.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE input_key=$1 OR manifest->'objects' @> jsonb_build_object('thumbnail',$1::text) OR manifest->'objects' @> jsonb_build_object('preview',$1::text) OR manifest->'objects' @> jsonb_build_object('metadata',$1::text)) OR EXISTS(SELECT 1 FROM job_attempts a JOIN jobs j ON j.id=a.job_id WHERE $1 LIKE a.output_prefix || '%' AND j.state='running' AND j.lease_token=a.token AND j.lease_expires_at>clock_timestamp())`, key).Scan(&yes)
	return yes, err
}
func (q *Queue) Metrics(ctx context.Context) (string, error) {
	var queued, running, succeeded, failed, attempts, retries, errorsCount int64
	var waitSum, processSum float64
	var finished int64
	err := q.DB.QueryRow(ctx, `SELECT count(*) FILTER(WHERE state='queued'),count(*) FILTER(WHERE state='running'),count(*) FILTER(WHERE state='succeeded'),count(*) FILTER(WHERE state='failed') FROM jobs`).Scan(&queued, &running, &succeeded, &failed)
	if err != nil {
		return "", err
	}
	err = q.DB.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE number>1),count(*) FILTER(WHERE outcome NOT IN ('running','succeeded')),COALESCE(sum(extract(epoch FROM a.started_at-j.created_at)) FILTER(WHERE number=1),0),COALESCE(sum(extract(epoch FROM a.ended_at-a.started_at)),0),count(*) FILTER(WHERE ended_at IS NOT NULL) FROM job_attempts a JOIN jobs j ON a.job_id=j.id`).Scan(&attempts, &retries, &errorsCount, &waitSum, &processSum, &finished)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# TYPE clipflow_jobs gauge\nclipflow_jobs{state=\"queued\"} %d\nclipflow_jobs{state=\"running\"} %d\nclipflow_jobs{state=\"succeeded\"} %d\nclipflow_jobs{state=\"failed\"} %d\n# TYPE clipflow_attempts_total counter\nclipflow_attempts_total %d\n# TYPE clipflow_retries_total counter\nclipflow_retries_total %d\n# TYPE clipflow_attempt_failures_total counter\nclipflow_attempt_failures_total %d\n# TYPE clipflow_queue_wait_seconds summary\nclipflow_queue_wait_seconds_sum %f\nclipflow_queue_wait_seconds_count %d\n# TYPE clipflow_processing_seconds summary\nclipflow_processing_seconds_sum %f\nclipflow_processing_seconds_count %d\n", queued, running, succeeded, failed, attempts, retries, errorsCount, waitSum, attempts-retries, processSum, finished), nil
}
