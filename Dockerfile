FROM golang:1.26.8-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && CGO_ENABLED=0 go build -o /out/cleanup ./cmd/cleanup

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl && rm -rf /var/lib/apt/lists/*
ARG FFMPEG_VERSION=5.1.9-0+deb12u1
RUN ffmpeg -version | head -n 1 | grep -F "ffmpeg version ${FFMPEG_VERSION} "
COPY --from=build /out/ /usr/local/bin/
USER 65534:65534
ENTRYPOINT ["api"]

FROM runtime AS verify
USER root
COPY --from=build /usr/local/go /usr/local/go
COPY --from=build /go/pkg/mod /go/pkg/mod
ENV PATH=/usr/local/go/bin:$PATH
ENV CGO_ENABLED=0
ENV GOPATH=/go
WORKDIR /src
COPY . .
ENTRYPOINT ["go"]
CMD ["test","-count=1","-v","./..."]
