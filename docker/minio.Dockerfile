FROM golang:1.26.8-bookworm AS build
RUN GOBIN=/out go install github.com/minio/minio@RELEASE.2025-09-07T16-13-09Z
FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/minio /usr/local/bin/minio
ENTRYPOINT ["minio"]
