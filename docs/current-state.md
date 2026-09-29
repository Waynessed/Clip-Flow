# Current state

## Delivered

CF-06 public read-only walkthrough is deployed at
https://waynessed.github.io/Clip-Flow/. Pages workflow 36593150641 passed for
63da9efa5408b2a9910434102f157d790d9d80c5. Actual HTTPS/browser checks verified
playback, four recorded scenarios, read-only requests, mobile width and all ten
exported asset hashes. It has no public API/upload/storage credentials.
See public-walkthrough.md. Local services were stopped when CF-06 began;
DB, MinIO and API were started temporarily to export the preserved examples,
then stopped with data volumes retained. No local services are left running.

CF-00–CF-05 implemented and verified locally. First demo and full kill/recovery/obsolete-publication demo work. The local app starts at http://localhost:5173; API http://localhost:8080. PostgreSQL and MinIO remain internal. Nine measured runs completed 900 unique jobs successfully, with 100 distinct fixtures and zero failed runs.

## Revisions

- b7bc6cf6b0e9f1aaa078039ddc755c94b7194874: final CF-05 documentation checkpoint.
- 63da9efa5408b2a9910434102f157d790d9d80c5: deployed CF-06 walkthrough implementation. The local Go application remains based on d1434ab. A follow-up records live verification and preserves byte-verified assets across Windows checkouts; use `git rev-parse HEAD` for the exact repository revision.

- e82c287: initial code and reproducible environment.
- 5bdefd5: first verified local/browser demo evidence.
- 721e75c0054a8de1224232f3559f7561db7b71dd: verified recovery/reliability code and benchmark runtime; remote clean-checkout CI passed.
- d1434ab135940b6a8cdf394e747b8b53bd235400: completed CF-05 local application implementation, final tests and benchmark/evidence records, pushed to origin/master.

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

- CF-06: public/local walkthrough builds; original frontend build; two local Playwright checks; two remote walkthrough checks and successful Pages deployment. Actual hosted preview playback and ten HTTPS asset hashes passed. Evidence: pages-deployment.json, public-walkthrough-live.json and walkthrough-playwright-local.json under docs/evidence.
- The separate original full-stack CI run 36593150644 for 63da9ef also completed successfully: clean-checkout Compose startup, Go tests/vet, frontend build and real upload/playback. Evidence: docs/evidence/ci-public-extension.json.

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

No required implementation/deployment work remains. Intentional limits: local
single workspace with no accounts or hosted upload backend; the public site
serves clearly labelled recorded examples only. Local execution can repeat with
guarded publication; orphan objects remain until explicit 24-hour cleanup; the
local API streams full objects without range support. Slow initial source
build, synthetic benchmark and sampled-resource limitations remain. The sampler
correction was validated from saved data, without inventing a repeated benchmark.
Run local service-interruption demos one at a time. Public-host verification and
rollback/unpublish commands are documented in public-walkthrough.md.
