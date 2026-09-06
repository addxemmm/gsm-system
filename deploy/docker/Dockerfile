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
#   ./scripts/prefetch_vendor.sh   (runs ON the server, fills third_party/)
# third_party/ is server-side only: untracked by git, never synced from Windows.
# Only FPGA images are reused from the legacy image (versionless data files).
#
# Stages:
#   legacy         -> FPGA images only (gsmsystem-dep:1.3, local, no pull)
#   uhd-builder    -> UHD release_003_009_000 from third_party/
#   openbts-builder -> liba53 + libcoredumper + openbts + smqueue
#                     (zeromq via stock xenial libzmq3-dev, same as legacy)
#   go-builder     -> Go single binary (replaces Python/Flask/gunicorn)
#   runtime        -> binaries + runtime libs + asterisk + seeds. NO python.
ARG OPENBTS_REV=master
ARG UHD_REV=release_003_009_000

# ---------- stage 0: legacy FPGA images only ----------
# NOTE: was gsmsystem-dep:1.3 (GC'd on host 2026-09-06); 1.0 carries the
# identical 2019 B210 FPGA file. Keep the 1.0 image until v2 is RF-verified.
FROM gsmsystem-dep:1.0 AS legacy

# ---------- stage 1: UHD ----------
FROM ubuntu:16.04 AS uhd-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake pkg-config python python-mako \
    libboost-all-dev libusb-1.0-0-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*
# Vendor source must match UHD_REV (see prefetch_vendor.sh).
COPY third_party/uhd /src/uhd
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
    libzmq3-dev wget ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=uhd-builder /usr/local /usr/local
RUN ldconfig
WORKDIR /src
# liba53 (A5/3) + libcoredumper, per upstream .travis.yml build recipe.
COPY third_party/liba53 ./liba53
COPY third_party/libcoredumper ./libcoredumper
RUN cd liba53 && dpkg-buildpackage -us -uc && cd .. && dpkg -i liba53*.deb \
 && cd libcoredumper && ./build.sh && dpkg -i *.deb
# openbts: submodules already present via host-side --recursive clone,
# so clone.sh/switchto.sh (network) are intentionally skipped.
COPY third_party/openbts ./openbts
WORKDIR /src/openbts
RUN ./autogen.sh \
 && ./configure \
 && make -j$(nproc) \
 && make install \
 && ldconfig
# smqueue: standalone sister repo, same autotools flow as openbts
# (installs to /usr/local/sbin like sipauthserve).
# libosip2-dev is installed here (not top apt) to keep iteration cached.
COPY third_party/smqueue ./smqueue
RUN apt-get update && apt-get install -y --no-install-recommends libosip2-dev && rm -rf /var/lib/apt/lists/* \
 && cd smqueue && ./autogen.sh && ./configure && make -j$(nproc) && make install && ldconfig
# subscriberRegistry (sipauthserve) also left the openbts tree upstream.
COPY third_party/subscriberRegistry ./subscriberRegistry
RUN cd subscriberRegistry && ./autogen.sh && ./configure && make -j$(nproc) && make install && ldconfig
# Debian-packaged libs land in /usr/lib*; normalize into /usr/local/lib
# so the runtime COPY below is deterministic (fail loudly if absent).
RUN for f in /usr/lib/liba53* /usr/lib/libcoredumper* /usr/lib/x86_64-linux-gnu/liba53* /usr/lib/x86_64-linux-gnu/libcoredumper*; do [ -e "$f" ] && cp -P "$f" /usr/local/lib/; done; ldconfig; ls /usr/local/lib/liba53* /usr/local/lib/libcoredumper*
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
    libsqlite3-0 libortp9 libusb-1.0-0 libreadline6 libzmq5 \
    libboost-date-time1.58.0 libboost-filesystem1.58.0 libboost-regex1.58.0 \
    libboost-system1.58.0 libboost-thread1.58.0 libboost-serialization1.58.0 \
    libboost-program-options1.58.0 \
    asterisk sqlite3 iptables iproute2 usbutils curl ca-certificates psmisc \
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
