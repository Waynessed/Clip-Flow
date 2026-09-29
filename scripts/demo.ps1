param([switch]$SkipBootstrap,[switch]$Recovery,[switch]$Stale)
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:DOCKER_CONTEXT='desktop-linux'
if (-not $SkipBootstrap) { & "$PSScriptRoot/bootstrap.ps1"; if ($LASTEXITCODE -ne 0) { throw 'Bootstrap failed' } }
New-Item -ItemType Directory -Force .artifacts | Out-Null
$container=docker compose ps -q api
docker compose exec -T api ffmpeg -nostdin -v error -y -f lavfi -i 'testsrc2=size=960x540:rate=24' -f lavfi -i 'sine=frequency=440:sample_rate=44100' -t 3 -c:v libx264 -threads 1 -pix_fmt yuv420p -c:a aac /tmp/demo.mp4
if ($LASTEXITCODE -ne 0) { throw 'Synthetic clip generation failed' }
docker cp "${container}:/tmp/demo.mp4" .artifacts/demo.mp4
$key=[guid]::NewGuid().ToString()
$json=curl.exe -sS -f -H "Idempotency-Key: $key" -F 'file=@.artifacts/demo.mp4' http://localhost:8080/v1/jobs
if ($LASTEXITCODE -ne 0) { throw 'Upload failed' }
$job=$json | ConvertFrom-Json
Write-Host "Submitted job $($job.id). Open http://localhost:5173 to watch."
$repeat=curl.exe -sS -f -H "Idempotency-Key: $key" -F 'file=@.artifacts/demo.mp4' http://localhost:8080/v1/jobs | ConvertFrom-Json
if ($repeat.id -ne $job.id) { throw 'Idempotency check failed' }
$deadline=(Get-Date).AddMinutes(3)
do { $detail=Invoke-RestMethod "http://localhost:8080/v1/jobs/$($job.id)"; Write-Host $detail.state; if ($detail.state -eq 'succeeded') { $detail | ConvertTo-Json -Depth 10; break }; if ($detail.state -eq 'failed') { throw $detail.error_message }; Start-Sleep -Seconds 1 } while ((Get-Date) -lt $deadline)
if ($detail.state -ne 'succeeded') { throw 'Processing did not complete before timeout' }
$detail | ConvertTo-Json -Depth 15 | Set-Content .artifacts/first-demo.json
if ($Recovery) { & "$PSScriptRoot/recovery.ps1" -Mode kill }
if ($Stale) { & "$PSScriptRoot/recovery.ps1" -Mode stale }
