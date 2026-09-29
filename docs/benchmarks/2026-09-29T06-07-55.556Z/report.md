# Measured local benchmark

Recorded 2026-09-29T06:21:05.539Z. Revision 721e75c0054a8de1224232f3559f7561db7b71dd plus working benchmark scripts.

100 distinct clips per run, 1/2/4 workers, three repetitions each. Workers limited to one CPU and 512 MiB each. Docker engine allocation: 8 logical CPUs / 7.648 GiB. See environment.json for host CPU, exact Compose and engine data; fixtures.json for input hashes.

| Workers | Run | Status | Completed | Jobs/min | Queue median / p95 (s) | E2E median / p95 (s) | Preview reduction | Mean / peak CPU (%) | Peak total memory (MiB) |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 1 | succeeded | 100 | 50.14 | 78.27 / 115.38 | 79.25 / 116.07 | 62.36% | 97.14 / 111.49 | 60.21 |
| 1 | 2 | succeeded | 100 | 53.20 | 60.11 / 107.03 | 60.93 / 107.69 | 62.36% | 96.58 / 104.30 | 59.09 |
| 1 | 3 | succeeded | 100 | 38.52 | 57.19 / 138.79 | 60.38 / 142.51 | 62.36% | 98.82 / 131.28 | 71.54 |
| 2 | 1 | succeeded | 100 | 145.40 | 29.57 / 39.19 | 30.18 / 39.75 | 62.36% | 188.32 / 192.20 | 79.78 |
| 2 | 2 | succeeded | 100 | 159.16 | 21.48 / 35.12 | 22.17 / 35.73 | 62.36% | 182.94 / 192.85 | 71.05 |
| 2 | 3 | succeeded | 100 | 133.15 | 18.85 / 40.96 | 19.58 / 42.39 | 62.36% | 190.01 / 203.66 | 72.23 |
| 4 | 1 | succeeded | 100 | 185.40 | 22.95 / 30.59 | 23.93 / 31.43 | 62.36% | 411.92 / 448.05 | 193.74 |
| 4 | 2 | succeeded | 100 | 90.57 | 22.83 / 59.80 | 25.36 / 62.71 | 62.36% | 392.83 / 469.64 | 161.50 |
| 4 | 3 | succeeded | 100 | 66.11 | 50.58 / 82.46 | 53.89 / 86.38 | 62.36% | 405.56 / 484.41 | 111.88 |

## Method and limits

All 100 uploads are staged with workers stopped and four API requests in flight. Throughput elapsed time starts immediately before Compose starts workers and ends when polling sees all jobs terminal; it includes worker startup and polling overhead. Queue wait is first attempt start minus database creation time, including batch staging. End-to-end is publication minus database creation, so upload validation/storage time before insertion is excluded; raw request timestamps also permit measuring client-observed upload overhead. CPU/memory samples are docker stats snapshots about every two seconds; CPU sums can exceed 100% across workers. Memory strings and per-container samples are in every raw run file.

This is a warm-cache synthetic workload on one laptop/WSL2 VM, no audio, one-second clips. It does not predict production throughput, longer inputs, cold builds, network storage, or saturated concurrent upload workloads. Database/object storage share host resources. No benchmark speedup is assumed. Failed runs, if any, are retained with reasons.

## Comparison across three repetitions

Each worker configuration completed 300/300 (1 workers), 300/300 (2 workers), 300/300 (4 workers). Aggregate throughput below is total successful jobs divided by summed run elapsed time. Median/p95 pool all completed jobs across each configuration's three runs.

| Workers | Jobs/min | Queue median / p95 (s) | E2E median / p95 (s) | Mean / peak CPU (%) | Peak total memory (MiB) | Preview / combined media reduction |
|---|---|---|---|---|---|---|
| 1 | 46.37 | 66.06 / 117.30 | 67.50 / 118.61 | 97.66 / 131.28 | 71.54 | 62.36% / 46.21% |
| 2 | 145.13 | 23.96 / 39.18 | 24.77 / 39.75 | 186.90 / 203.66 | 79.78 | 62.36% / 46.21% |
| 4 | 95.05 | 27.92 / 75.75 | 29.42 / 78.63 | 402.02 / 484.41 | 193.74 | 62.36% / 46.21% |

Measured throughput ratios relative to one worker: two workers 3.13×; four workers 2.05×. These ratios describe this recorded synthetic batch on one shared laptop, including the scheduling/polling costs stated above. They do not establish a production scaling guarantee. Combined media reduction includes preview + JPEG and excludes the small metadata JSON. Resource values are sampled, so short peaks can be missed.

Failed runs: 0. All raw runs are retained.

Two workers had the highest aggregate throughput. Four-worker run rates ranged from 66.11 to 185.40 jobs/minute; this wide variation is retained. Host load, power state and thermal behaviour were not controlled or measured, so the cause of that variation is not established. The unexpectedly large two-vs-one ratio also needs controlled follow-up before attributing it to worker scaling. CPU snapshots briefly exceed configured per-worker quota equivalents; these are Docker-reported observations, not proof of sustained quota violations. Individual and pooled resource summaries are derived from the persisted raw samples; summary-original.json preserves the original in-memory summary before the sampler serialization correction.

Run order was grouped and fixed: three one-worker runs, then three two-worker runs, then three four-worker runs. It was not randomized; order/cache/power-state effects are additional limitations of this recorded comparison.
