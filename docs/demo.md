# Local demo

From a clean checkout in PowerShell with Docker Desktop running its Linux engine:

```powershell
./scripts/bootstrap.ps1
./scripts/demo.ps1 -SkipBootstrap
```

Open http://localhost:5173. Choose `.artifacts/demo.mp4` or your own short MP4, click Process clip, and inspect the preview, thumbnail, metadata and attempt history. The request key remains available to repeat the same upload. Progress is polled every second; a very short job can finish between polls.

The demo script generates a real 3-second 960x540 clip plus audio inside the API container, copies it to `.artifacts`, uploads it twice with the same key and checks identity, then polls completion. Outputs are proxied through port 8080 or Vite's same-origin proxy. First build includes source compilation of pinned MinIO because public image registries refused access in this environment.

Stop preserving data: `docker --context desktop-linux compose down`. Explicit destructive reset of local demo data: `docker --context desktop-linux compose down -v` (only when you intend to discard all clips). Cleanup: `docker --context desktop-linux compose run --rm --entrypoint cleanup tools`.

Troubleshooting: bootstrap prints build errors or service logs. Check Linux engine, registry access, localhost ports 5173/8080 and `docker compose logs api worker`. Recovery demo and exact measured timings will be added after verification.
