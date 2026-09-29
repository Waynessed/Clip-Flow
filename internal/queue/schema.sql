CREATE TABLE IF NOT EXISTS jobs (
 id uuid PRIMARY KEY,
 idempotency_key text NOT NULL UNIQUE,
 input_hash text NOT NULL,
 input_key text NOT NULL,
 filename text NOT NULL,
 state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','running','succeeded','failed')),
 attempt_count integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_owner text,
 lease_token uuid,
 lease_expires_at timestamptz,
 manifest jsonb,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 error_category text,
 error_message text
);
CREATE INDEX IF NOT EXISTS jobs_claim ON jobs (available_at,created_at) WHERE state IN ('queued','running');
CREATE TABLE IF NOT EXISTS job_attempts (
 job_id uuid NOT NULL REFERENCES jobs(id),
 number integer NOT NULL,
 token uuid NOT NULL UNIQUE,
 worker text NOT NULL,
 started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 ended_at timestamptz,
 outcome text NOT NULL DEFAULT 'running',
 error_category text,
 error_message text,
 output_prefix text NOT NULL,
 PRIMARY KEY(job_id,number)
);
