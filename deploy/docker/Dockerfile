# syntax=docker/dockerfile:1
# gsm-system v2 FRESH image (no Python): OpenBTS + Asterisk + Go API.
# Rebuilt from RangeNetworks/openbts source + local gsmsystem configs,
# replacing the 6.25GB legacy image (which carried full source trees,
# apt build-deps and docker-commit snapshots).
#
# Base is Ubuntu 16.04 Xenial ON PURPOSE: OpenBTS 5.0-era code links
# libortp.so.9 / boost-1.58 / icu55 and pairs with Asterisk 11-13 chan_sip.
# Stock archive.ubuntu.com still serves xenial (verified 2026).
#
# HERMETIC SOURCE: xenial's TLS stack can no longer handshake github.com,
# so builders do ZERO git/network. Sources are prefetched on the HOST
# (whose git works) before building:
#   ./scripts/prefetch_vendor.sh   (runs ON the server, fills vendor/)
# vendor/ is server-side only: untracked by git, never synced from Windows.
# Only FPGA images are reused from the legacy image (versionless data files).
#
# Stages:
#   legacy         -> FPGA images only (gsmsystem-dep:1.3, local, no pull)
#   uhd-builder    -> UHD release_003_009_000 from vendor/
#   openbts-builder -> libzmq (source) + liba53 + libcoredumper + openbts + smqueue
#   go-builder     -> Go single binary (replaces Python/Flask/gunicorn)
#   runtime        -> binaries + runtime libs + asterisk + seeds. NO python.
ARG OPENBTS_REV=master
ARG UHD_REV=release_003_009_000

# ---------- stage 0: legacy FPGA images only ----------
FROM gsmsystem-dep:1.3 AS legacy

# ---------- stage 1: UHD ----------
FROM ubuntu:16.04 AS uhd-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake pkg-config python python-mako \
    libboost-all-dev libusb-1.0-0-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*
# Vendor source must match UHD_REV (see prefetch_vendor.sh).
COPY vendor/uhd /src/uhd
WORKDIR /src/uhd/host/build
RUN cmake ../ -DCMAKE_BUILD_TYPE=Release \
 && make -j$(nproc) \
 && make install \
 && ldconfig

# ---------- stage 2: zmq + OpenBTS + smqueue + sipauthserve ----------
FROM ubuntu:16.04 AS openbts-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential autoconf automake libtool pkg-config dpkg-dev debhelper \
    sqlite3 libsqlite3-dev libortp-dev libusb-1.0-0-dev libreadline-dev \
    ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=uhd-builder /usr/local /usr/local
RUN ldconfig
WORKDIR /src
# libzmq from source (replaces the dead chris-lea PPA path in install_libzmq.sh).
COPY vendor/libzmq ./libzmq
RUN cd libzmq && ./autogen.sh && ./configure && make -j$(nproc) && make install && ldconfig
# liba53 (A5/3) + libcoredumper, per upstream .travis.yml build recipe.
COPY vendor/liba53 ./liba53
COPY vendor/libcoredumper ./libcoredumper
RUN cd liba53 && dpkg-buildpackage -us -uc && cd .. && dpkg -i liba53*.deb \
 && cd libcoredumper && ./build.sh && dpkg -i *.deb
# openbts: submodules already present via host-side --recursive clone,
# so clone.sh/switchto.sh (network) are intentionally skipped.
COPY vendor/openbts ./openbts
WORKDIR /src/openbts
RUN ./autogen.sh \
 && ./configure \
 && make -j$(nproc) \
 && make install \
 && ldconfig
# smqueue is NOT part of the normal build: standalone makefile (see README).
RUN make -f Makefile.standalone -C smqueue || (cd smqueue && make -f Makefile.standalone) \
 && cp smqueue/smqueue /usr/local/sbin/smqueue || cp apps/smqueue /usr/local/sbin/smqueue
RUN ls -l /OpenBTS/OpenBTS /OpenBTS/transceiver /usr/local/sbin/sipauthserve /usr/local/sbin/smqueue

# ---------- stage 3: Go API server (replaces Python/Flask) ----------
FROM golang:1.22-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/gsm-system ./cmd/server

# ---------- stage 4: slim runtime (NO python) ----------
FROM ubuntu:16.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 libortp9 libusb-1.0-0 libreadline6 \
    libboost-date-time1.58.0 libboost-filesystem1.58.0 libboost-regex1.58.0 \
    libboost-system1.58.0 libboost-thread1.58.0 libboost-serialization1.58.0 \
    asterisk sqlite3 iptables iproute2 usbutils curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*
# Fresh-built stack (/usr/local carries UHD + zmq + a53 + coredumper + daemons).
COPY --from=openbts-builder /usr/local /usr/local
# B210 FPGA image reused from legacy (data file, no rebuild needed).
COPY --from=legacy /usr/local/share/uhd/images /usr/local/share/uhd/images
COPY --from=openbts-builder /OpenBTS/OpenBTS /OpenBTS/transceiver /OpenBTS/OpenBTSCLI /OpenBTS/OpenBTSDo /OpenBTS/
COPY --from=openbts-builder /usr/local/sbin/sipauthserve /usr/local/sbin/smqueue /usr/local/sbin/comp128 /usr/local/sbin/
RUN ldconfig \
 && ln -sf /usr/local/sbin/sipauthserve /OpenBTS/sipauthserve \
 && ln -sf /usr/local/sbin/smqueue /OpenBTS/smqueue
# Local authoritative configs: asterisk dialplan/SIP + clean-DB seeds.
# (gsmsystem/asterisk/*.conf are the tuned set exported from the legacy cell.)
COPY gsmsystem/asterisk/ /etc/asterisk/
COPY configs/seeds/TMSITable_init.db configs/seeds/sqlite3_init.db /OpenBTS/
COPY configs/seeds/run.so /OpenBTS/run.so
COPY --from=openbts-builder /src/openbts/apps/OpenBTS.example.sql /app/seeds/OpenBTS.example.sql
# Go server + bundled configs (no gunicorn, no Flask, no python).
COPY --from=go-builder /out/gsm-system /usr/local/bin/gsm-system
COPY configs/ /app/configs/
RUN cp /app/configs/app.yaml.example /app/configs/app.yaml \
 && mkdir -p /data/conf /data/log /etc/OpenBTS /var/run /var/lib/asterisk/sqlite3dir
COPY deploy/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /OpenBTS/OpenBTSDo

ENV GSM_CONFIG=/app/configs/app.yaml \
    GSM_DATA_DIR=/data \
    UHD_IMAGES_DIR=/usr/local/share/uhd/images

EXPOSE 8082
ENTRYPOINT ["/entrypoint.sh"]
