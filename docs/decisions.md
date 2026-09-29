# Decisions

- PostgreSQL queue: durable state and short `FOR UPDATE SKIP LOCKED` claim transactions avoid a second queue system. Processing happens after commit.
- Ownership: every claim gets a random UUID token. Database clock determines validity. Heartbeat and publication both require the current token and an unexpired lease. Worker identity alone cannot distinguish old and new claims.
- Publication: outputs have unique attempt prefixes. A conditional job update and attempt completion share one DB transaction. Execution may repeat; only a valid current attempt can publish. This is not exactly-once execution.
- Retry: transient storage/local I/O errors retry after 5 then 15 seconds; maximum three claims including expired ones. Invalid input is permanent. Defaults are 60-second lease, 10-second heartbeat, 120-second processing deadline.
- Upload consistency: stream to disk, validate/hash, check key, store object, insert job. Unique idempotency key resolves concurrent submissions. A losing request may leave an orphan input. No shared S3/PostgreSQL transaction is claimed.
- Cleanup: explicit command, 24-hour minimum age, per-object DB reference checks; refuse deletion on database errors. Active attempt objects are also protected.
- UI direction: slate blue workspace (#e8eef4), paper panels (#f7fafd), ink (#203248), processing blue (#285cc7), success teal (#22634f). Segoe UI emphasizes readable controls. Clip list on the left, a large video frame and attempt timeline on the right; stack on mobile. The video frame is the main visual element.
- Benchmark design: 100 real synthetic clips, 1/2/4 workers, three runs each with identical input and one CPU/512 MiB per worker. Raw evidence is in docs/benchmarks; all nine runs completed. The report states uncontrolled-host and sampled-resource limitations.

- CF-06 public scope: the user requested a read-only walkthrough, so GitHub Pages serves a separate React static build containing only reviewed synthetic assets and recorded scenarios. An always-running public Go/DB/worker stack would add compute, secrets, visitor data and operational responsibilities without enabling any requested public action. Public uploads are a future scope change.
- Public correctness: show recorded timestamps and outcomes explicitly; never simulate a new queued/running job. Source links use the actual CF-05 implementation revision. SHA-256 verification detects media drift before deployment.
- Deployment: the existing GitHub repository and Actions provide Pages hosting over HTTPS without a new paid account. Only the static artifact is uploaded; PRs verify but do not deploy. Deploy permissions are restricted to the deployment job and no long-lived hosting token is committed.

## Verified revisions

- e82c287: initial implementation with queue leases/publication primitives included alongside the first vertical path.
- 5bdefd5: first verified browser/media demo evidence.
- 721e75c: local reliability/recovery/cleanup verification and evidence.

The stage order prioritized the first working demo. Several later-stage primitives were implemented in the initial code to avoid rewriting the schema; evidence and demos for those primitives were delivered after first-demo acceptance. This is a documented scheduling deviation, not a claim that untested early code was verified.

The verification image includes Go and FFmpeg and mounts source for local iteration. Its explicit Go module path and persistent compiler cache avoid repeated downloads/recompilation. Integration tests isolate PostgreSQL schemas and MinIO buckets instead of truncating the live demo queue.

The UI uses the Vite same-origin proxy. Object content is served through API output routes rather than presigned MinIO URLs, keeping the storage service internal and limiting access to the published manifest. Output HTTP range support is currently omitted; the bounded short previews were verified to play with full-object streaming.
