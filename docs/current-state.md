# Current state

CF-00 through CF-04: verified local implementation. CF-05: tests/CI/cleanup implemented, nine benchmarks pending. First demo and complete kill/recovery/stale-publication demo work. Local UI http://localhost:5173, API http://localhost:8080.

Current recorded HEAD: 5bdefd5 (first-demo evidence). The next checkpoint contains reliability/verification code; read git rev-parse HEAD for current revision, and implementation-log.md plus git log for the code/evidence history.

Commands: ./scripts/bootstrap.ps1; ./scripts/demo.ps1 -SkipBootstrap -Recovery -Stale; node scripts/reliability.mjs; ./scripts/test.ps1; ./scripts/benchmark.ps1; docker --context desktop-linux compose run --rm --entrypoint cleanup tools.

Prerequisites: Docker Desktop Linux engine, PowerShell; Node 24 for host browser tests/benchmark runner. No host Go or FFmpeg. Initial internet build compiled pinned upstream MinIO because registries refused images; first source build took 483 seconds. Database/storage are internal, API/frontend localhost only.

Verified: empty-DB migration/readiness, real media flow, actual browser playback, idempotency including ten concurrent duplicates, exclusive claims, three-attempt retry/expiry bounds, heartbeat renewal/cancellation, real killed-worker recovery, actual obsolete completion rejected, real API/worker restarts preserving queued work, real MinIO outage with bounded retries, cleanup unit and real-store reference protection, metrics query, Go vet. Evidence: docs/evidence.

Unverified: remote GitHub Actions execution, complete nine-run benchmark, clean checkout replay after the latest packaging changes. Next: commit recovery checkpoint, run benchmark, final clean-start validation and document exact results.
