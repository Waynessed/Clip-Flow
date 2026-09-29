$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:DOCKER_CONTEXT='desktop-linux'
New-Item -ItemType Directory -Force .artifacts | Out-Null
docker compose --profile test build verify
if ($LASTEXITCODE -ne 0) { throw 'Test image build failed' }
docker compose --profile test run --rm verify test -count=1 -v ./... 2>&1 | Tee-Object .artifacts/go-tests.log
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
docker compose --profile test run --rm verify vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
& "$PSScriptRoot/demo.ps1" -SkipBootstrap
Push-Location web
try { npm ci; if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }; npx playwright install chromium; if ($LASTEXITCODE -ne 0) { throw 'Browser install failed' }; npm run build; if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }; npm run test:e2e; if ($LASTEXITCODE -ne 0) { throw 'Browser test failed' } } finally { Pop-Location }
