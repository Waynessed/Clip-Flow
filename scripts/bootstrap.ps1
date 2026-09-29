param([int]$Workers=1)
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:DOCKER_CONTEXT='desktop-linux'
docker info *> $null
if ($LASTEXITCODE -ne 0) { throw 'Start Docker Desktop with its Linux engine, then rerun scripts/bootstrap.ps1.' }
docker compose up -d --build --scale "worker=$Workers"
if ($LASTEXITCODE -ne 0) { docker compose logs --tail 50; throw 'Compose startup failed. Inspect the build error above and check registry access.' }
$deadline=(Get-Date).AddMinutes(3)
do {
  try { $ready=Invoke-RestMethod http://localhost:8080/readyz -TimeoutSec 3; $web=Invoke-WebRequest http://localhost:5173 -TimeoutSec 3; if ($ready.status -eq 'ready' -and $web.StatusCode -eq 200) { Write-Host 'ClipFlow ready: http://localhost:5173'; return } } catch {}
  Start-Sleep -Seconds 2
} while ((Get-Date) -lt $deadline)
docker compose ps
docker compose logs --tail 60 api worker db minio
throw 'Readiness did not succeed within 3 minutes. Check database/storage health and port conflicts.'
