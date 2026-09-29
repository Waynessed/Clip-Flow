 `CLIPFLOW_HANDOFF.md`

```markdown
# ClipFlow — Implementation Handoff

## 1. Instruction to the implementing Codex session

Implement this project in the current repository. This repository is dedicated
to ClipFlow and is independent of the job-search repository.

Priority:
1. Deliver a working local end-to-end demo.
2. Add the reliability mechanisms and demonstrate recovery.
3. Complete tests, benchmarks and implementation documentation.

Proceed through the stages below without waiting for approval for routine
implementation choices. Preserve existing user changes.

The user wants to learn how the project was implemented later. Maintain the
implementation records described below as you work. Do not rely on chat history
as the only record.

Never describe planned behaviour, simulated output or unexecuted tests as
completed work.

## 2. Product and success criteria

Build a video-processing platform that accepts a short MP4 and produces:
- A JPEG thumbnail.
- An MP4 preview, maximum height 480 pixels.
- Metadata: duration, dimensions, input/output sizes.
- A visible job state and processing-attempt history.

The final demo must show:
1. Upload a clip.
2. Watch queued → running → succeeded.
3. View generated outputs.
4. Submit the same request twice and receive the same job.
5. Kill a processing worker.
6. Show another worker recovering the job.
7. Show that an obsolete attempt cannot publish its result.

First-demo acceptance:
- Real input, real FFmpeg processing and real persistent storage.
- Browser UI can upload, display progress and open outputs.
- Startup instructions work from a clean checkout.
- No placeholder success screens or hard-coded processing results.

## 3. Fixed scope and stack

Backend: Go, standard-library HTTP routing, pgx.
Database: PostgreSQL.
Media processing: FFmpeg and ffprobe.
Object storage: MinIO through the AWS SDK for Go v2 S3 client.
Frontend: React, TypeScript and Vite.
Environment: Linux containers through Docker Compose.
Tests: Go tests, real-service integration tests, Playwright smoke test.
CI: GitHub Actions.

Choose compatible stable dependency versions during setup and pin them in
lockfiles/container configuration. Record exact versions in the implementation log.

Use one Go module with separate API and worker entrypoints.
Keep business logic separate from HTTP handlers and storage adapters.

Architecture:
Browser → API → PostgreSQL job queue
              → MinIO input objects
Workers → PostgreSQL claim/lease
        → MinIO input
        → FFmpeg
        → MinIO attempt outputs
        → PostgreSQL guarded result publication

Local ports:
- Frontend: 5173.
- API: 8080.
- Bind exposed services to localhost.
- PostgreSQL and MinIO remain on the internal Docker network.

Single-workspace application. No accounts or multitenancy.
No Kubernetes, Kafka, payments, live streaming, mobile app or public deployment.
No arbitrary remote media URLs.

## 4. Public interfaces

POST /v1/jobs
- Multipart upload containing one "file".
- Require Idempotency-Key header.
- Maximum 20 MB, maximum 30 seconds, supported MP4 with a video stream.
- Enforce upload size before loading the entire file into memory.
- Use ffprobe with a bounded timeout to inspect actual media.
- Return 202 with job ID and current state for a new job.
- Same key + same input hash: return the original job.
- Same key + different input hash: return 409.
- Invalid media: return a clear 4xx error.

GET /v1/jobs
- Most recent 50 jobs, newest first.

GET /v1/jobs/{id}
- State, timestamps, attempt count, processing history, error category,
  metadata and output links.

GET /v1/jobs/{id}/outputs/{kind}
- kind is thumbnail, preview or metadata.
- Serve only the published output manifest.
- Proxy object content through the API so the browser does not need MinIO access.

GET /healthz
- Process health.

GET /readyz
- Database and storage connectivity.

GET /metrics
- Prometheus-compatible queue, processing, retry and outcome metrics.

Use UUIDs, UTC timestamps and a consistent JSON error envelope.
Document the API in a checked-in OpenAPI description.

## 5. Data and correctness rules

Tables:
- jobs: identity, idempotency key, input hash/object key, state,
  attempt count, next available time, lease owner/token/expiry,
  published output manifest, timestamps and final error.
- job_attempts: job, attempt number/token, worker identity,
  start/end times, outcome, error category and output prefix.

States:
queued → running → succeeded
                 → queued for retry
                 → failed permanently

Upload sequence:
1. Stream upload to a bounded temporary file.
2. Validate and hash it.
3. Check idempotency.
4. Store input object.
5. Insert durable job.
6. Acknowledge only after database success.

Use database uniqueness to resolve concurrent requests with the same key.
A failed insert can leave an unreferenced input object; cleanup handles it.
Do not pretend object storage and PostgreSQL share one atomic transaction.

Worker rules:
- Claim inside a short transaction using FOR UPDATE SKIP LOCKED.
- Do not keep the transaction open while running FFmpeg.
- Give each claim a unique attempt token.
- Default lease: 60 seconds; renew every 10 seconds.
- Use database time for lease decisions.
- Heartbeats and publication require the current token and unexpired lease.
- If lease ownership is lost, cancel that worker's processing.
- Reclaim expired attempts and record their expiry.
- Maximum three attempts, including expired attempts.
- Retry transient failures after 5 seconds, then 15 seconds.
- Corrupt/unsupported input is a permanent failure.

Write outputs under a unique attempt prefix.
Publish the manifest only through a conditional database update for the current
attempt. Stale workers may leave objects, but cannot change the published result.

State the guarantee honestly:
Execution may repeat after failures. Only the current valid attempt can publish
the job's visible result. This is not exactly-once processing.

Media rules:
- Fixed FFmpeg arguments passed without shell interpolation.
- Maximum processing time: 120 seconds.
- Thumbnail from the first usable frame.
- Preserve preview aspect ratio; use even dimensions, H.264/yuv420p,
  and AAC when audio exists.
- Keep input access local; prohibit network fetching.
- Publish success only when all outputs exist and metadata validates.

Cleanup:
- Explicit CLI command, not an automatic destructive startup action.
- Remove unreferenced inputs and unsuccessful-attempt objects older than 24 hours.
- Never delete when reference checks fail or PostgreSQL is unavailable.

## 6. Implementation stages

### CF-00 — Scaffold and reproducible environment
- Inspect existing files and available Docker/Go/Node tools.
- Create the project structure, lockfiles and Compose services.
- Add schema migrations and health checks.
- Create the implementation documentation.
- Add scripts/bootstrap.ps1 and scripts/demo.ps1.
- Bootstrap must wait for readiness and report actionable failures.

Acceptance: services start; readiness succeeds; migrations apply to an empty DB.

### CF-01 — First vertical demo
- Implement upload validation, input storage, job insertion.
- Implement one worker's claim and successful processing path.
- Implement list/detail/output endpoints.
- Implement upload form, job table and result preview.
- Generate small synthetic clips using FFmpeg for repeatable demos.

Acceptance: a real clip moves through the UI and produces valid outputs.
Deliver this demo immediately before adding extra features.

Planning estimate: CF-00 + CF-01 approximately 10–15 focused hours.

### CF-02 — Idempotency and bounded failures
- Add concurrent-safe idempotency.
- Add explicit permanent/transient error categories.
- Add timeouts, retry scheduling and maximum attempts.
- Add integration tests for duplicates, corrupt input and retry exhaustion.

Acceptance: duplicate requests do not create duplicate durable jobs;
invalid media never retries indefinitely.

### CF-03 — Leases and recovery
- Add heartbeats, expiry reclamation and attempt-token publication guards.
- Test worker termination and obsolete-worker completion.
- Add a deterministic integration-test failpoint to pause before publication.
- Keep failpoints disabled by default and clearly restricted to tests/demo mode.

Acceptance: recovered work completes; stale publication is rejected.

### CF-04 — Observability and recovery demo
- Structured JSON logs with job/attempt/worker identifiers.
- Metrics: queue depth, attempts, failures, queue wait and processing duration.
- UI shows attempt history and clear failure explanations.
- demo.ps1 submits a clip, kills one worker container, and polls recovery.
- Demo may use a documented shorter lease; production defaults remain unchanged.

Acceptance: the recovery demo is reproducible, visible and uses real processing.

### CF-05 — Verification and portfolio package
- Add Playwright upload-to-preview smoke test.
- Add cleanup tests.
- Benchmark one, two and four workers using 100 synthetic clips.
- Repeat each benchmark configuration three times.
- Record hardware/resource limits and all raw results.
- Finalise README, architecture diagram, walkthrough and demo instructions.

Total original scope estimate: approximately 55 hours.
Estimates are planning guidance, not a reason to stop with an incomplete demo.

## 7. Tests and measured evidence

Required scenarios:
- Same key/input repeated ten times gives one job.
- Same key/different input gives 409.
- Concurrent workers do not share an active attempt.
- Worker death leads to recovery after lease expiry.
- Obsolete worker cannot publish.
- API/worker restarts preserve queued work.
- Storage interruption causes bounded retries.
- Corrupt input causes a permanent failure.
- Output metadata describes real generated files.
- Cleanup preserves referenced objects.

Benchmark report:
- Completed jobs per minute.
- Median/p95 queue wait and end-to-end completion time.
- CPU/memory observations.
- Output-size reduction.
- One/two/four-worker comparison and limitations.

Do not invent performance improvements.
Include failed benchmark runs and reasons where relevant.

## 8. Implementation records for future learning

Create and maintain:

AGENTS.md
- Require future agents to read this plan and current-state/log documents.
- Require implementation records to remain aligned with actual changes.
- Require explanations to cite the relevant code revision.

docs/implementation-log.md
For every CF stage or meaningful correction, record:
- Step ID and UTC date.
- Goal.
- What actually changed.
- Relevant files and named functions/types.
- End-to-end control/data flow.
- Why this approach was chosen.
- Commands/tests executed and observed results.
- Limitations, deviations and remaining work.
- Local Git commit hash, when a commit is available.

docs/decisions.md
- Record queue choice, lease ownership, publication guards, retry policy,
  object/database consistency and benchmark design.
- Explain rejected alternatives briefly.

docs/walkthrough.md
- Explain actual upload, claim, heartbeat, processing, publication and recovery.
- Reference stable function names and relative file paths.
- Use small code excerpts where helpful.
- Update after changes; do not describe an earlier implementation as current.

docs/current-state.md
- Current implemented stage, actual HEAD, working commands, prerequisites,
  verified behaviour, unverified behaviour and next task.

docs/demo.md
- Exact startup, reset and demonstration commands.
- Expected visible results.
- Troubleshooting and normal timings.

After a verified stage, make a local commit if Git is available and writable.
Do not push remotely without user instruction.
If a commit cannot be made, record that explicitly.

At the end, report:
- First-demo status.
- Final-demo status.
- Exact launch/demo commands.
- Tests actually run.
- Implementation records and remaining limitations.

For a later question such as "Explain CF-03", read the log, relevant revision,
actual code and tests first. Explain what was really implemented, distinguishing
it from the original plan and any later modifications.

## 9. Primary references

PostgreSQL queue locking:
https://www.postgresql.org/docs/current/sql-select.html

FFmpeg:
https://ffmpeg.org/documentation.html

ffprobe:
https://ffmpeg.org/ffprobe.html
```