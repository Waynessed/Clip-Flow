# Implementation log

## CF-00 / CF-01 in progress — 2026-09-29 UTC

Goal: first real upload-to-preview demo, then reliability and verification.

Initial repository: only the untracked CLIPFLOW_HANDOFF.md, no commits. Origin is https://github.com/Waynessed/Clip-Flow.git. User explicitly requested commits and pushes. Host has Node 24.21.0, npm 11.19.0, Docker client/engine 29.8.0, Compose 5.5.1. Docker Desktop was stopped and was started. No host Go or FFmpeg. Linux container engine: 8 CPUs, 7.648 GiB, WSL2 kernel 6.18.33.2.

Actual changes: one Go module; API, worker and cleanup entrypoints; PostgreSQL schema embedded in queue.Migrate; AWS SDK storage.Store; bounded disk upload service.Service.Upload; media.Probe and media.Process; React workspace; Compose and PowerShell bootstrap/demo scripts. Queue implementation contains the lease/token primitives needed for later stages; recovery evidence is still pending.

Flow: browser multipart → bounded temporary file → ffprobe/hash → key lookup → unique MinIO input → durable PostgreSQL insert → worker claim → local download → FFmpeg outputs → unique attempt prefix → guarded manifest publication → API output proxy. Object upload and database insert are separate operations; orphan objects are possible.

Versions selected: Go 1.26.8, PostgreSQL 18.3, pgx 5.7.4, AWS core 1.36.3 / credentials 1.17.66 / S3 1.78.2, React 19.3.0, Vite 8.3.1, plugin-react 6.1.1, TypeScript 7.0.2, Playwright 1.63.0. Dependency resolution/builds pending. Debian runtime pinned by digest. FFmpeg exact package version will be recorded after build.

Executed: tool inventory, docker info, remote inventory (empty), npm version queries, Docker pulls. Go and PostgreSQL image pulls succeeded. Guessed Debian dated tag did not exist; replaced with verified digest. Both Docker Hub and Quay refused the MinIO release image. Switched to building the exact upstream MinIO RELEASE.2025-09-07T16-13-09Z from source, following upstream source-install guidance (https://github.com/minio/minio). This adds build time but retains the requested stack.

No end-to-end success claimed yet. No commit yet. Next: resolve dependencies, compile, start, run the real demo and browser smoke test.

Build checkpoint: Go compilation via container go test ./... succeeded before integration tests were added (packages had no tests then). npm ci and npm run build succeeded after adding vite/client declarations and explicitly typing the request-key state as string. npm audit reported zero vulnerabilities. Full Compose build is still running; MinIO source compilation and FFmpeg package installation are the main initial costs. Browser Chromium was installed. The draft real-service tests and recovery scripts are present but unexecuted at this checkpoint.

## CF-01 verified first demo — 2026-09-29 05:55 UTC

Implementation basis: e82c287 (pushed to origin/master). Full Compose build started an empty PostgreSQL 18.3 database and MinIO source build. API schema migration and bucket creation succeeded; /readyz passed. Initial MinIO source build took 483 seconds; subsequent builds reuse it. Runtime FFmpeg/ffprobe is 5.1.9-0+deb12u1. MinIO source release resolves to Go pseudo-version v0.0.0-20250907161309-07c3a429bfed; upstream go-install embeds DEVELOPMENT.GOGET in --version, so the build instruction and source revision identify it.

Executed ./scripts/demo.ps1 -SkipBootstrap: generated real 3-second 960x540 H.264/AAC input (547291 bytes), uploaded it twice with one key, verified the same job ID, observed success with one attempt. Job 3e5c1166-48a3-435e-9980-3a34ca926a41 produced an 854x480 H.264/AAC preview (196753 bytes, 3.019 seconds) and JPEG (19418 bytes). Real detail response preserved at docs/evidence/first-demo.json. Creation to publication was 3.478 seconds on the first job.

Executed npm run test:e2e in web: one Playwright browser smoke test passed (4.4 seconds test, 8.5 seconds suite). It uploaded via the form, loaded preview video metadata, checked dimensions/duration, read generated metadata and repeated the upload. Screenshot was reviewed: no layout clipping at 1280 pixels. Actual frame playback will be added to strengthen the smoke assertion. Browser opening requested in Codex (panel queued). Working local demo is available at http://localhost:5173.

Go compilation/cleanup tests passed; real-service integration tests were skipped in the plain Go container because TEST_DATABASE_URL was not set. No recovery success or benchmark evidence claimed yet. Next: execute isolated integration tests and recovery scripts, then benchmark.

## CF-02 — idempotency and bounded failures verified — 2026-09-29 06:05 UTC

Goal: make duplicate submissions and failures predictable. queue.ByKey/Insert use the unique key and SHA-256 comparison; Service.Upload uses fixed ffprobe timeout and disk bounds; queue.Fail enforces attempt limits and 5/15-second scheduling. HTTP handlers map validation, conflict and availability errors to the documented envelope. The database arbitrates concurrent insert races because a process-level mutex would not work across APIs. Upload losers can leave orphan inputs; cleanup handles those separately.

Executed real PostgreSQL/MinIO/FFmpeg suite TestRealServiceScenarios: ten sequential duplicates and ten concurrent matching-key uploads, different-input conflict, HTTP 400/409/413/422/202 contract, missing objects with scheduled delays and three-attempt exhaustion, and worker-side corrupt stored input with one permanent attempt all passed. Retry-delay tests inspect real SQL timestamps and then advance available_at rather than sleeping; real outage demo below exercises actual waits. Implementation code originally introduced in e82c287; current tests and corrections are captured by the next checkpoint commit.

## CF-03 — leases and recovery verified — 2026-09-29 06:05 UTC

Goal: bound ownership and prevent obsolete publication. queue.Claim/Heartbeat/Publish/Fail use database time and token guards; Worker.Handle cancels work on failed renewal; outputs live under Prefix(job). Short row-lock transactions avoid holding locks throughout FFmpeg. Expired attempts count toward the same maximum of three. The restricted publication barrier was extended with DEMO_ABANDON_HEARTBEAT for the obsolete-worker demo only; it requires all demo flags, affects only first attempts, and does not bypass publication guards. Marker creation errors are now handled and marker files are removed after the attempt.

Executed concurrent claim test (eight claimers, one active attempt), expired heartbeat/publication rejection, new-token obsolete publication rejection, three expired-attempt exhaustion, heartbeat renewal past original two-second expiry and cancellation after forced loss. Real-service tests passed (seven scenarios, 9.63 seconds); cleanup unit tests passed. Go vet passed. Commands: docker compose --profile test run --rm -e GOPATH=/go verify test -count=1 -v ./...; same service with vet ./.... The original verify image lacked GOPATH=/go and downloaded modules again; Dockerfile now sets it and uses a persistent test build cache. Source is mounted for local test iterations, separate schema/bucket prevent demo-job consumption.

Executed scripts/recovery.ps1 -Mode kill: actual Docker worker kill after real outputs were written, expiry then another worker succeeded. Job 682f3d8e-9d1e-41c1-93f9-3f5beb5894d6 completed in 11.46 seconds with expired → succeeded history and different worker IDs. Executed -Mode stale: first attempt deliberately stopped renewal at the real publication barrier; second worker published; releasing the old attempt produced stale_publication and failure_record_rejected logs with unchanged manifest. A repeated stale run also passed; latest evidence is job 6f616aa0-2a1c-4b41-867d-80de1cd63433. JSON and logs are in docs/evidence.

## CF-04 — observability and complete recovery demonstration verified — 2026-09-29 06:05 UTC

Goal: make real state/retries explainable. Worker/API JSON logs, durable queue.Metrics counters/timing summaries, React attempt history/error display, exact PowerShell recovery scripts and scripts/reliability.mjs are implemented. Default leases remain 60/10 seconds; only recovery containers use 8/2 seconds. Browser polls every second, so very short states can occur between polls.

Executed node scripts/reliability.mjs: stopped worker, uploaded queued work, restarted actual API, read unchanged queued job, started worker and observed success. Then stopped MinIO with another queued job; actual network/DNS failure caused three attempts with real 5/15-second waits and final storage_unavailable. Storage and workers were restored in finally. Evidence docs/evidence/reliability.json. Metrics SQL was executed in the real-service suite. No exactly-once guarantee is claimed.

## CF-05 — verification package underway — 2026-09-29 06:05 UTC

Added cleanup.Sweep so cleanup can be tested independently, with a complete reference pass plus a final per-object recheck. Unit tests prove age/namespace/reference preservation and refusal on DB failure. Real-store test uses a documented test-only future cutoff to make fresh objects eligible; it deletes an orphan while preserving published objects, durable input and active-attempt objects. The CLI always uses real now minus 24 hours. Cleanup is never automatic at startup.

Strengthened Playwright to actually play the MP4, observe currentTime advancing beyond 0.2 seconds, pause it, and capture the screenshot. Passed (13.8-second test / 16.5-second suite, concurrent integration compilation on the laptop). Production frontend build already passed. GitHub Actions workflow added for Compose, integration/vet and browser tests with evidence upload; remote execution is not yet verified. Benchmark scripts prepared for 100 distinct one-second 960x540 hue variants, nine runs (1/2/4 workers, three each), raw details, CPU/memory samples, input hashes and environment capture. Benchmarks have not been run at this checkpoint.
