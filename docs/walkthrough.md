# Implementation walkthrough

Code basis: reliability checkpoint `721e75c`; earlier upload/processing code is in `e82c287`, and the first-demo evidence checkpoint is `5bdefd5`. Read the implementation log and `git log` for later packaging/benchmark changes. Paths/function names below describe actual code.

## 1. Browser to durable job

`web/src/main.tsx` uses a real multipart file and request key. `request<T>` parses the JSON error envelope. `App.upload` submits the file and selects the returned job. The refresh effect polls the list and selected detail every second; outputs appear only on succeeded jobs.

`internal/httpapi/API.upload` requires `Idempotency-Key` and exactly one multipart field named file. `http.MaxBytesReader` caps the full request at 20 MiB plus 64 KiB framing allowance. `osTemp` copies at most 20 MiB plus one byte to disk; it checks extra parts before creating durable work. This avoids acknowledging a request with unexpected multipart fields. There are currently two bounded temporary-file copies: the HTTP adapter validates form cardinality and the service owns validation/hash/storage. It is a small deliberate duplication at this scale and avoids keeping HTTP concerns in business logic.

`internal/service/Service.Upload` streams the file to a private temporary file while hashing SHA-256, invokes `media.Probe`, then calls `queue.ByKey`. A matching hash returns the original job; a different hash conflicts. Otherwise it writes a UUID input object, then calls `queue.Insert`. `INSERT ... ON CONFLICT(idempotency_key) DO NOTHING` plus a second key/hash lookup resolves concurrent uploads across API processes. The service acknowledges only after the database insert/read succeeds. A concurrent losing request or failed insertion can leave an orphan object. PostgreSQL and object storage do not share an atomic transaction.

## 2. Claim transaction

`cmd/worker/main.go` creates one worker identity from hostname plus UUID and calls `Worker.Run`. `internal/queue/Queue.Claim` selects a queued eligible job or expired running job:

```sql
... FOR UPDATE SKIP LOCKED LIMIT 1
```

The transaction locks only one job while recording ownership. An expired previous attempt is ended with outcome expired. If three claims are already consumed, the job permanently fails. Otherwise a fresh UUID token, owner, lease expiry and incremented attempt count are stored, and a `job_attempts` row is inserted. Commit happens before downloading or running FFmpeg. Concurrent claimers skip a locked row rather than all waiting for it.

The default lease is 60 seconds; renewals occur every ten seconds. `clock_timestamp()` determines eligibility and validity in PostgreSQL, avoiding host-clock comparisons. A stopped worker loses ownership when its lease expires; its attempt still counts toward the three-attempt limit.

## 3. Heartbeat and cancellation

`internal/worker/Worker.Handle` creates a 120-second attempt context and starts the renewal goroutine. `Queue.Heartbeat` requires all three ownership conditions:

```sql
lease_token=$2 AND state='running'
AND lease_expires_at>clock_timestamp()
```

A rejected heartbeat or database error cancels the context. `exec.CommandContext` and S3 calls share this context, so loss of ownership stops processing promptly. Even if cancellation is delayed or a stale process completes, publication has its own independent database guard. The test `heartbeat_renews_then_cancels_lost_ownership` observes a live lease beyond its original expiry, forces loss, checks cancellation, then processes recovery with a fresh token.

## 4. Local media processing

`Worker.process` creates a private work directory and downloads the input through `storage.Store.Download`. `media.Probe` invokes ffprobe with a ten-second timeout and file/pipe-only protocol whitelist. It checks an actual MP4 brand, a video stream, positive duration up to 30 seconds, and dimensions bounded to 7680×4320.

`media.Process` passes a fixed argument array directly to FFmpeg, with no shell. It creates the first usable video frame as a JPEG and a preview with even dimensions, height at most 480, H.264/yuv420p, optional AAC audio and faststart. Explicit single-thread settings bound per-process work and match benchmark resource limits. The generated MP4 is inspected again with media.Inspect; the JPEG header is decoded. Inspect permits finite positive output duration, including small AAC padding beyond the input-only 30-second limit. media.Probe retains the upload limit. File sizes and real duration/dimensions go into `model.Manifest`.

Both audio and no-audio inputs were processed in executed tests/demos. Output metadata is compared to a separately downloaded/probed preview in `real_outputs_metadata_and_restart`. The browser smoke test plays the actual MP4 and observes advancing playback time.

## 5. Attempt objects and publication

`Queue.Prefix` returns `attempts/<job UUID>/<attempt token>/`. All three outputs are stored under that unique prefix, including the metadata JSON. Successful S3 PutObject acknowledgements precede publication. Input and output sizes come from the actual files, never hard-coded responses.

`Queue.Publish` performs a conditional job update inside a transaction using the same token/state/expiry conditions as heartbeat. Exactly one updated row is required. The transaction also ends the matching attempt as succeeded. A stale token or expired lease returns `queue.ErrStale` without changing the visible manifest. Readers serve only that manifest through `API.output`; MinIO remains private to the Docker network.

Execution can repeat. Only the current valid attempt can publish the visible result. This is not exactly-once execution. Rejected attempts may leave invisible objects for cleanup.

## 6. Failures and recovery

`Queue.Fail` uses the publication ownership guard before changing state. Transient errors schedule queued work after five seconds on attempt 1 and fifteen seconds on attempt 2; attempt 3 becomes failed. Corrupt stored input is a permanent invalid_media failure. Processing timeout is transient and bounded by the same maximum. Every claim, including expired work, counts.

`API.upload` rejects invalid media before insertion with 422, oversized input with 413, conflicting key/content with 409, and missing fields/key with 400. Worker logs include job ID, attempt number/token and worker ID. `Queue.Metrics` reads durable state/attempt counts and queue-wait/processing duration summaries from PostgreSQL. A reset of demo tables resets these persisted totals; they are not process-local uptime counters.

`scripts/recovery.ps1` stops the normal worker, runs a demo worker, and waits at a real after-output publication barrier. Kill mode terminates it and observes another worker reclaim after expiry. Stale mode explicitly enables the demo-only abandon-renewal flag, waits for recovery, then releases the old process and verifies stale publication rejection and unchanged manifest. Normal worker configuration has none of these flags; database publication checks are identical in all modes. Barrier marker files are cleaned up after attempts.

`scripts/reliability.mjs` exercises actual API/worker restarts preserving queued work and an actual stopped MinIO service causing three failed attempts. Restoration runs in finally. Evidence files record real IDs, timestamps, outcomes and log messages.

## 7. Explicit cleanup

`cmd/cleanup/main.go` invokes `internal/cleanup/Sweep` with `now - 24 hours`. Sweep lists objects, considers only inputs/attempts older than the cutoff, and completes a database reference pass before any deletion. It rechecks each object immediately before deleting it. `Queue.Referenced` protects durable job inputs, every published object and objects under a current unexpired attempt prefix. Listing/reference errors stop cleanup. Startup never deletes media.

Unit tests cover the 24-hour cutoff, namespace filtering, reference preservation and unavailable database. Real-service coverage supplies a documented test-only future cutoff so new fixture objects are eligible without waiting a day; it deletes a real orphan and verifies referenced inputs, published outputs and active-attempt objects remain. The production CLI has no cutoff override.

## 8. Verification and measurement

`internal/integration/integration_test.go` creates a unique schema and bucket, runs actual adapters and FFmpeg, then removes only its test resources. Queue tests advance database timestamps explicitly where a fast deterministic scheduling assertion is useful. The kill, stale and outage scripts separately demonstrate real wall-clock behaviour.

`web/tests/upload.spec.ts` uploads through the UI, checks generated links, plays the preview, compares video/metadata dimensions, repeats the same request and checks browser errors. `.github/workflows/ci.yml` starts real services, runs Go tests/vet and Playwright, and retains logs/artifacts. Local results and remote Actions status are recorded separately.

`scripts/benchmark.ps1` generates 100 distinct one-second hue variants using real FFmpeg. `benchmark.mjs` submits all 100 with four requests in flight while workers are stopped, starts 1/2/4 workers, records terminal job details and docker stats, and repeats each configuration three times. Raw files retain attempts/timestamps/metadata, failures, input hashes and environment/resource limits. Throughput includes worker startup and polling; queue wait includes batch staging; database-created-to-published latency excludes pre-insertion upload work. The benchmark report states these limits before drawing comparisons.
