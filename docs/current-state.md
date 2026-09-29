# Current state

## Delivered

CF-06 public read-only walkthrough is implemented and verified locally; its
first Pages deployment is pending. It uses actual exported synthetic media
and four archived scenarios, with no public API/upload/storage credentials.
See public-walkthrough.md. Local services were stopped when CF-06 began;
DB, MinIO and API were started temporarily to export the preserved examples,
then stopped with data volumes retained. No local services are left running.

CF-00–CF-05 implemented and verified locally. First demo and full kill/recovery/obsolete-publication demo work. The local app starts at http://localhost:5173; API http://localhost:8080. PostgreSQL and MinIO remain internal. Nine measured runs completed 900 unique jobs successfully, with 100 distinct fixtures and zero failed runs.

## Revisions

- b7bc6cf6b0e9f1aaa078039ddc755c94b7194874: final CF-05 documentation checkpoint.
- CF-06 implementation revision will be recorded after its verified commit.

- e82c287: initial code and reproducible environment.
- 5bdefd5: first verified local/browser demo evidence.
- 721e75c0054a8de1224232f3559f7561db7b71dd: verified recovery/reliability code and benchmark runtime; remote clean-checkout CI passed.
- d1434ab135940b6a8cdf394e747b8b53bd235400: completed CF-05 implementation, final local tests and all benchmark/evidence records, pushed to origin/master. This is the current implementation HEAD. A documentation-only follow-up records final CI status; use `git rev-parse HEAD` for the exact current documentation revision.

## Commands

```powershell
./scripts/bootstrap.ps1
./scripts/demo.ps1 -SkipBootstrap -Recovery -Stale
node scripts/reliability.mjs
./scripts/test.ps1
./scripts/benchmark.ps1
# Explicit reference/age-protected cleanup
 docker --context desktop-linux compose run --rm --entrypoint cleanup tools
```

Requirements: Docker Desktop Linux engine and PowerShell. Host Node 24 is needed for Playwright/benchmarks only. No host Go/FFmpeg required. Internet is required for the first build; pinned MinIO source compilation took 483 seconds here, subsequent builds reuse it.

## Executed verification

- Fresh database migration/bucket initialization and readiness; final bootstrap also passed.
- Real MP4 upload, JPEG/854x480 H.264/AAC preview, actual metadata and browser playback.
- Ten sequential and ten concurrent identical-key requests; different-content 409; malformed/oversized/corrupt request bounds.
- Exclusive claims; three-attempt retry/expiry bounds; 5/15-second retry scheduling; corrupt stored input permanent failure.
- Heartbeat renewal and ownership-loss cancellation; actual killed-worker recovery; actual obsolete completion rejected with unchanged manifest.
- Actual API/worker restarts preserving queued work; real MinIO outage exhausting exactly three attempts, services restored.
- Cleanup age/namespace/reference/refusal unit tests and real orphan deletion/reference protection.
- Input 30.000 seconds accepted with 30.001-second AAC output; 31-second input rejected.
- Go vet, frontend build, final Playwright upload/playback, git diff --check.
- Nine benchmarks; all raw evidence preserved. Aggregate throughput 46.37/145.13/95.05 jobs per minute for 1/2/4 workers. Two workers fastest in aggregate; four-worker variance and uncontrolled host conditions explicitly reported.
- Remote clean-checkout GitHub Actions passed both 721e75c (run 36529397777) and final implementation d1434ab (run 36531496192). Final evidence: docs/evidence/ci-final.json; https://github.com/Waynessed/Clip-Flow/actions/runs/36531496192.

## Records and limits

Read implementation-log.md, walkthrough.md, decisions.md, demo.md and benchmark-report.md. Original and final evidence is in docs/evidence; measured raw data is in docs/benchmarks. Explanations of stages must read the actual recorded revision and tests.

No required implementation work remains. Intentional limits: local single workspace, no accounts/public deployment, repeated execution possible with guarded publication, orphan objects retained until explicit 24-hour cleanup, full-object streaming without HTTP range support, slow initial source build, short synthetic benchmark and sampled resources. The sampler reporting correction was validated against saved raw data; the full benchmark was not rerun after that correction. Normal service/demos should be run one at a time when intentionally stopping workers/storage.
