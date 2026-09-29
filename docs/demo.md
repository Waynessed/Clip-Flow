# Demonstration guide

## Public read-only walkthrough

https://waynessed.github.io/Clip-Flow/ (initial deployment status is recorded in
current-state.md). Select one of the four recorded runs, play the actual preview
or original input, open metadata/thumbnail, inspect attempts and follow the
processing stages. These are recorded examples, not newly submitted jobs.

For local preview, run `npm run build:walkthrough` then
`npm run preview:walkthrough` in web; open http://127.0.0.1:4173/Clip-Flow/.
Ctrl+C stops that preview. The hosted copy is independent of Docker and the
local computer. See public-walkthrough.md for unpublishing and rollback.

## First working local demo

PowerShell, repository root, Docker Desktop Linux engine running:

```powershell
./scripts/bootstrap.ps1
./scripts/demo.ps1 -SkipBootstrap
```

Bootstrap builds images, starts persistent PostgreSQL/MinIO, waits for API and frontend readiness, and prints logs on failure. Allow 10–15 minutes for an uncached first build on this laptop/network; the MinIO source compilation alone took 483 seconds. The scripts use `desktop-linux` context. No host Go or FFmpeg required.

Open http://localhost:5173. Select `.artifacts/demo.mp4`, click Process clip, follow queued/running/succeeded, and open the thumbnail/preview/metadata. The 3-second 960x540 test input produced an 854x480 preview in about one to four seconds after claim here. The UI polls every second, so short states can occur between refreshes. Outputs and history are real persistent data. Repeating the upload with the same request key returns the same job. Changing the file generates a new UI key; manually reuse the previous key with different valid content to see a conflict.

## Complete recovery demonstration

```powershell
./scripts/demo.ps1 -SkipBootstrap -Recovery -Stale
```

Or individual stages:

```powershell
./scripts/recovery.ps1 -Mode kill
./scripts/recovery.ps1 -Mode stale
node scripts/reliability.mjs
```

Run service-interruption demos one at a time on an idle workspace. They stop normal workers before placing a job, so the first claim belongs to the demo worker. The script waits until FFmpeg and real object writes finish at a publication barrier.

Kill mode kills that container. A normal worker claims after the eight-second demo lease expires and succeeds with attempt 2. History shows expired then succeeded and different worker identities. The measured kill job finished in 11.46 seconds from creation. The script restores normal workers in finally.

Stale mode explicitly enables CLIPFLOW_DEMO_MODE=1, PAUSE_BEFORE_PUBLISH=1 and DEMO_ABANDON_HEARTBEAT=1 on only that demo container. Attempt 1 stops renewal and waits with completed real outputs. Another worker reclaims and publishes. The script then releases attempt 1 and checks for stale_publication plus an unchanged visible manifest. Production/default containers have no failpoint flags. The publication guard is never bypassed.

`reliability.mjs` stops the worker, uploads queued media, restarts the actual API, checks the durable queue, then starts the worker and observes success. It also stops actual MinIO while another job is queued: three network-failure attempts with 5/15-second waits end in storage_unavailable. Finally it restores MinIO and the worker. That outage intentionally creates one failed job visible in the UI.

Responses/logs are retained in `.artifacts/`; original verified runs are checked in under `docs/evidence/`.

## Tests and measurement

```powershell
./scripts/test.ps1
./scripts/benchmark.ps1
```

The test script assumes a bootstrapped app, builds the Go verification image, runs real-service tests/vet, regenerates a demo fixture, builds the frontend and runs Playwright. Tests have separate DB schemas and buckets. Host Node 24 is needed for Playwright. The benchmark creates 100 one-second 960x540 hue variants and records nine runs with resource samples in `docs/benchmarks/`. It leaves the app with one normal worker. Do not submit unrelated jobs while benchmarking.

## Stop, cleanup and reset

```powershell
# Preserve database and media
 docker --context desktop-linux compose down
# Explicit cleanup: unreferenced inputs/unsuccessful-attempt objects older than 24h
 docker --context desktop-linux compose run --rm --entrypoint cleanup tools
# Destructive reset only when intentionally discarding the local workspace
 docker --context desktop-linux compose down -v
```

Cleanup preserves all durable input references, published objects and current unexpired attempt objects. It refuses to proceed if object listing or database reference checks fail. Fresh orphans remain until the 24-hour threshold.

## Troubleshooting

- Engine stopped: start Docker Desktop and select Linux containers; verify `docker --context desktop-linux info`.
- Build unavailable: check internet/registries/Go module proxy. MinIO is source-built from its pinned release, avoiding the registries that refused the image here.
- Port conflict: free localhost 5173/8080 and rerun bootstrap. No host PostgreSQL/MinIO ports are required.
- Not ready: `docker --context desktop-linux compose ps` and `docker --context desktop-linux compose logs --tail 100 api worker db minio`.
- Upload rejected: use supported MP4 with a video stream, at most 30 seconds and 20 MiB. File extension alone is insufficient.
- Recovery container name conflict after an interrupted shell: inspect `docker --context desktop-linux ps -a`, remove only the matching `clipflow-kill-demo` or `clipflow-stale-demo` container, and rerun. Do not use remove-orphans against an unrelated active demo.
- Queue stalled: inspect attempts and worker logs; after a worker dies the default recovery delay is up to its 60-second lease expiry. Demo leases are shorter.
- Database volumes on disk persist across down/up. Reset is never an automatic startup action.
