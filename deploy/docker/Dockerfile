# syntax=docker/dockerfile:1
# gsm-system v2 (Go API + OpenBTS reuse strategy).
# RISK NOTE: rebuilding OpenBTS/UHD on Ubuntu 22.04 is NOT attempted here
# (upstream master + UHD 003_009 needs old toolchain; >2 rounds forbidden).
# Primary path: reuse the proven gsmsystem-dep:1.3 layer, replace only the
# Python/Flask API with the Go single binary + seeds. Old container/image
# stay on the host for rollback.
ARG BASE_IMAGE=gsmsystem-dep:1.3

FROM golang:1.22-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/gsm-system ./cmd/server

FROM ${BASE_IMAGE}
ENV DEBIAN_FRONTEND=noninteractive
# sqlite3 + iproute2/iptables needed by the Go server; keep layer tiny.
RUN apt-get update && apt-get install -y --no-install-recommends \
    sqlite3 iproute2 iptables curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=go-builder /out/gsm-system /usr/local/bin/gsm-system
COPY configs/ /app/configs/
RUN cp /app/configs/app.yaml.example /app/configs/app.yaml || true
COPY deploy/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && mkdir -p /data/conf /data/log
ENV GSM_CONFIG=/app/configs/app.yaml GSM_DATA_DIR=/data
EXPOSE 8082
ENTRYPOINT ["/entrypoint.sh"]
