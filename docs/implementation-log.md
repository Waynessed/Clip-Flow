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
