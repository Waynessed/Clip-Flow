$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:DOCKER_CONTEXT='desktop-linux'
New-Item -ItemType Directory -Force .artifacts/benchmark-clips | Out-Null
docker compose exec -T api sh -c 'mkdir -p /tmp/benchmark-clips; i=0; while [ "$i" -lt 100 ]; do ffmpeg -nostdin -v error -y -f lavfi -i "testsrc2=size=960x540:rate=12" -vf "hue=h=$i" -t 1 -c:v libx264 -threads 1 -pix_fmt yuv420p "/tmp/benchmark-clips/clip-$i.mp4" || exit 1; i=$((i+1)); done'
if ($LASTEXITCODE -ne 0) { throw 'Benchmark fixture generation failed' }
$api=docker compose ps -q api
docker cp "${api}:/tmp/benchmark-clips/." .artifacts/benchmark-clips
if ($LASTEXITCODE -ne 0) { throw 'Could not copy benchmark fixtures' }
node scripts/benchmark.mjs
if ($LASTEXITCODE -ne 0) { throw 'Benchmark contains failed runs; inspect raw evidence' }

node scripts/report-benchmark.mjs
if ($LASTEXITCODE -ne 0) { throw 'Benchmark comparison report failed' }
