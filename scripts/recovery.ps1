param([ValidateSet('kill','stale')][string]$Mode='kill')
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:DOCKER_CONTEXT='desktop-linux'
if (-not (Test-Path .artifacts/demo.mp4)) { & "$PSScriptRoot/demo.ps1" -SkipBootstrap }
function Check-Docker { if ($LASTEXITCODE -ne 0) { throw 'Docker operation failed' } }
function Wait-Job([string]$id,[string]$state) {
  $deadline=(Get-Date).AddMinutes(3)
  do { $j=Invoke-RestMethod "http://localhost:8080/v1/jobs/$id"; if ($j.state -eq $state) { return $j }; if ($j.state -eq 'failed') { throw $j.error_message }; Start-Sleep -Seconds 1 } while ((Get-Date) -lt $deadline)
  throw "Job did not reach $state"
}
$name="clipflow-$Mode-demo"
docker compose stop worker
Check-Docker
try {
  $args=@('compose','run','-d','--no-deps','--name',$name,'-e','CLIPFLOW_DEMO_MODE=1','-e','PAUSE_BEFORE_PUBLISH=1','-e','LEASE_SECONDS=8','-e','HEARTBEAT_SECONDS=2')
  if ($Mode -eq 'stale') { $args+=@('-e','DEMO_ABANDON_HEARTBEAT=1') }
  $args+=@('--entrypoint','worker','tools')
  & docker @args
  Check-Docker
  $key=[guid]::NewGuid().ToString()
  $raw=curl.exe -sS -f -H "Idempotency-Key: $key" -F 'file=@.artifacts/demo.mp4' http://localhost:8080/v1/jobs
  Check-Docker
  $job=$raw | ConvertFrom-Json
  $marker="/tmp/clipflow-paused-$($job.id)"
  $deadline=(Get-Date).AddSeconds(60)
  do { docker exec $name test -f $marker 2>$null; if ($LASTEXITCODE -eq 0) { break }; Start-Sleep -Milliseconds 250 } while ((Get-Date) -lt $deadline)
  if ($LASTEXITCODE -ne 0) { throw 'Worker did not reach the publication barrier' }
  Write-Host "First attempt paused for $($job.id). Open http://localhost:5173 to watch history."
  if ($Mode -eq 'kill') { docker kill $name; Check-Docker; Write-Host 'Worker killed. Waiting for another worker to reclaim its lease.' }
  docker compose up -d --no-deps worker
  Check-Docker
  $result=Wait-Job $job.id 'succeeded'
  if ($result.attempt_count -ne 2 -or $result.attempts[0].outcome -ne 'expired') { throw 'Recovery history mismatch' }
  if ($result.attempts[0].worker -eq $result.attempts[1].worker) { throw 'Recovery did not use another worker' }
  if ($Mode -eq 'stale') {
    $manifest=$result.metadata | ConvertTo-Json -Depth 10 -Compress
    docker exec $name touch "$marker.release"
    Check-Docker
    $deadline=(Get-Date).AddSeconds(15)
    do { $logs=docker logs $name 2>&1 | Out-String; if ($logs.Contains('stale_publication')) { break }; Start-Sleep -Milliseconds 250 } while ((Get-Date) -lt $deadline)
    if (-not $logs.Contains('stale_publication')) { throw 'Obsolete publication rejection was not observed' }
    $after=Invoke-RestMethod "http://localhost:8080/v1/jobs/$($job.id)"
    if (($after.metadata | ConvertTo-Json -Depth 10 -Compress) -ne $manifest) { throw 'Obsolete attempt changed published results' }
    Write-Host 'Obsolete worker finished, but its publication was rejected.'
  }
  New-Item -ItemType Directory -Force .artifacts | Out-Null
  $result | ConvertTo-Json -Depth 15 | Set-Content ".artifacts/recovery-$Mode.json"
  docker logs $name 2>&1 | Set-Content ".artifacts/recovery-$Mode.log"
  $result | ConvertTo-Json -Depth 10
} finally {
  docker rm -f $name 2>$null | Out-Null
  docker compose up -d --no-deps worker
}
