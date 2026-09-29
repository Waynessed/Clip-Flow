# Implementation walkthrough

This document describes current code; execution evidence lives in implementation-log.md.

## Upload
`internal/httpapi.API.upload` requires the idempotency header and exactly one multipart file. `osTemp` limits disk copying to 20 MB + 1 byte and checks extra parts before job creation. `internal/service.Service.Upload` validates with `media.Probe`, computes SHA-256 while streaming, checks `queue.ByKey`, then stores a unique input and calls `queue.Insert`. The database unique constraint arbitrates concurrent keys; matching hashes return the original job, different hashes conflict.

## Claim and heartbeat
`queue.Claim` locks one eligible row using SKIP LOCKED. Expired attempts are ended as expired. Three exhausted claims permanently fail the job. Otherwise a fresh token and attempt record are created in the same short transaction. `worker.Worker.Handle` starts a processing deadline and heartbeat goroutine; `queue.Heartbeat` rejects obsolete/expired ownership. A heartbeat error cancels the processing context.

## Media and publication
`worker.process` downloads the input to a private temporary directory. `media.Probe` uses local-only ffprobe and a ten-second bound. `media.Process` creates a first-frame JPEG and an even-dimension H.264/yuv420p preview up to 480 pixels high, with optional AAC audio. It probes real preview metadata and decodes the JPEG header. Outputs go under `queue.Prefix(job)`, which includes the claim token. `queue.Publish` changes visible state/manifest only while the same token has a live lease, then ends the attempt in that transaction. HTTP output routes read only this published manifest and proxy MinIO content.

## Failure and recovery
`queue.Fail` guards ownership exactly like publication, clears the lease and either schedules a bounded retry or ends the job. A killed worker cannot heartbeat. Another claim after database expiry ends its attempt and takes a new token. Old objects remain invisible. The explicit demo-only `PAUSE_BEFORE_PUBLISH` file barrier can pause the first attempt after writing objects; it is enabled only with `CLIPFLOW_DEMO_MODE=1`.

## Cleanup
The cleanup CLI lists objects, filters age and namespace, and calls `queue.Referenced` before each deletion. It stops on reference-check errors. Startup never deletes media.
