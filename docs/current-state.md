# Current state

First demo: verified and running at http://localhost:5173. CF-00/CF-01 implementation revision: e82c287, pushed to origin/master. The next documentation commit records this checkpoint; use git rev-parse HEAD for the exact current documentation revision.

Working commands: ./scripts/bootstrap.ps1; ./scripts/demo.ps1 -SkipBootstrap. Prerequisites: Docker Desktop Linux engine, PowerShell and internet for the first build. Node is needed only for host Playwright tests, not for the container demo. No host Go/FFmpeg needed. API on localhost:8080, frontend on localhost:5173; PostgreSQL/MinIO internal only.

Verified: fresh database migration, bucket creation, readiness, real upload, matching-key deduplication, JPEG/854x480 MP4/metadata, browser upload and video metadata loading. Frontend production build passes. Go code compiles; cleanup unit tests pass. Recorded first job at docs/evidence/first-demo.json.

Current work: CF-02–CF-05. Real-service tests, recovery/stale publication demos, cleanup integration coverage, CI and benchmarks need execution. New test and cleanup files in the working tree are not yet part of the verified demo revision.
