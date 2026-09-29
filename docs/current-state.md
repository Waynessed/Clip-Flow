# Current state

Stage: CF-00 / CF-01 implementation, verification pending. HEAD: unborn master (no commit yet).

Prerequisites: Docker Desktop Linux engine and PowerShell; first build needs internet. Host Go/FFmpeg are unnecessary. Run `./scripts/bootstrap.ps1`, then `./scripts/demo.ps1 -SkipBootstrap` from the repository. Frontend http://localhost:5173; API http://localhost:8080. Database/storage have no host ports.

Verified: Docker engine running; pinned Go and PostgreSQL images available. Unverified: compilation, startup, media flow, leases, cleanup and all tests. See implementation-log.md for actual dependency and registry issues.

Next task: build and deliver first working demo, then CF-02–CF-05 evidence.
