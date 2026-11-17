# syntax=docker/dockerfile:1

# ------------------------------------------------------------
# Frontend assets
# ------------------------------------------------------------

FROM node:24.20.0-bookworm-slim AS assets

WORKDIR /src

COPY package.json package-lock.json ./

RUN npm ci

COPY web/styles ./web/styles
COPY internal/webui/templates ./internal/webui/templates

RUN npm run assets:build


# ------------------------------------------------------------
# Go build
# ------------------------------------------------------------

FROM golang:1.27.0-bookworm AS builder

ENV GOMAXPROCS=4
ENV GOFLAGS="-p=4"


WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

COPY --from=assets \
  /src/web/static \
  ./web/static


ARG VERSION=dev
ARG REVISION=unknown
ARG BUILD_TIME=unknown


RUN CGO_ENABLED=0 \
  go test ./...


RUN CGO_ENABLED=0 \
  go build \
  -trimpath \
  -ldflags="\
  -s -w \
  -X github.com/maciejlistos/traillog/internal/buildinfo.Version=${VERSION} \
  -X github.com/maciejlistos/traillog/internal/buildinfo.Revision=${REVISION} \
  -X github.com/maciejlistos/traillog/internal/buildinfo.BuildTime=${BUILD_TIME}" \
  -o /out/traillog \
  ./cmd/traillog

RUN CGO_ENABLED=0 \
  go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /out/traillog-backup \
  ./cmd/traillog-backup

# Keep certificates for future HTTPS OTLP or other TLS clients.
RUN cp \
  /etc/ssl/certs/ca-certificates.crt \
  /out/ca-certificates.crt


# Create a directory that will also work for local container
# runs without Kubernetes.
RUN mkdir -p /out/data && \
  chown 65532:65532 /out/data


# ------------------------------------------------------------
# Runtime
# ------------------------------------------------------------

FROM scratch

WORKDIR /app


COPY --from=builder \
  /out/ca-certificates.crt \
  /etc/ssl/certs/ca-certificates.crt


COPY --from=builder \
  --chown=65532:65532 \
  /out/traillog \
  /app/traillog


COPY --from=assets \
  --chown=65532:65532 \
  /src/web/static \
  /app/web/static


COPY --from=builder \
  --chown=65532:65532 \
  /out/data \
  /data

COPY --from=builder \
  --chown=65532:65532 \
  /out/traillog-backup \
  /app/traillog-backup


USER 65532:65532


ENV TRAILLOG_ENVIRONMENT=prod
ENV TRAILLOG_HTTP_ADDRESS=:8080
ENV TRAILLOG_LOG_LEVEL=info
ENV TRAILLOG_LOG_FORMAT=json
ENV TRAILLOG_DATABASE_PATH=/data/traillog.db


EXPOSE 8080


ENTRYPOINT ["/app/traillog"]
