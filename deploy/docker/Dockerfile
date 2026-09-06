# syntax=docker/dockerfile:1
# gsm-system v2 FRESH image (no Python): OpenBTS + Asterisk + Go API.
# Rebuilt from RangeNetworks/openbts source + local gsmsystem configs,
# replacing the 6.25GB legacy image (which carried full source trees,
# apt build-deps and docker-commit snapshots).
#
# Base is Ubuntu 16.04 Xenial ON PURPOSE: OpenBTS 5.0-era code links
# libortp.so.9 / boost-1.58 / icu55 and pairs with Asterisk 11-13 chan_sip.
# Xenial is EOL, so apt is pointed at old-releases (see below).
# No PPAs: zeromq comes from NodeManager/install_libzmq.sh (upstream path).
#
# Stages:
#   uhd-builder  -> UHD release_003_009_000 + FPGA images
#   openbts-builder -> liba53 + libcoredumper + openbts master + smqueue
#   go-builder   -> Go single binary (replaces Python/Flask/gunicorn)
#   runtime      -> binaries + runtime libs + asterisk + seeds. NO python.
ARG OPENBTS_REV=master
ARG UHD_REV=release_003_009_000

# ---------- stage 1: UHD ----------
FROM ubuntu:16.04 AS uhd-builder
ARG UHD_REV
ENV DEBIAN_FRONTEND=noninteractive
RUN sed -i 's|http://archive.ubuntu.com|http://old-releases.ubuntu.com|g; s|http://security.ubuntu.com|http://old-releases.ubuntu.com|g' /etc/apt/sources.list \
 && apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake git pkg-config python python-mako \
    libboost-all-dev libusb-1.0-0-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone --recursive https://github.com/EttusResearch/uhd.git uhd \
 && cd uhd && git checkout ${UHD_REV} \
 && mkdir -p host/build && cd host/build \
 && cmake ../ -DCMAKE_BUILD_TYPE=Release \
 && make -j$(nproc) \
 && make install \
 && ldconfig
# FPGA images for B210 (builder has python; runtime only keeps the files).
RUN /usr/local/bin/uhd_images_downloader.py || /usr/local/lib/uhd/utils/uhd_images_downloader.py || true

# ---------- stage 2: OpenBTS + smqueue + sipauthserve ----------
FROM ubuntu:16.04 AS openbts-builder
ARG OPENBTS_REV
ENV DEBIAN_FRONTEND=noninteractive
RUN sed -i 's|http://archive.ubuntu.com|http://old-releases.ubuntu.com|g; s|http://security.ubuntu.com|http://old-releases.ubuntu.com|g' /etc/apt/sources.list \
 && apt-get update && apt-get install -y --no-install-recommends \
    build-essential autoconf automake libtool git pkg-config dpkg-dev \
    sqlite3 libsqlite3-dev libortp-dev libusb-1.0-0-dev libreadline-dev \
    libzmq3-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=uhd-builder /usr/local /usr/local
RUN ldconfig
WORKDIR /src
# liba53 (A5/3) + libcoredumper, per upstream .travis.yml build recipe.
RUN git clone https://github.com/RangeNetworks/liba53.git \
 && cd liba53 && dpkg-buildpackage -us -uc && cd .. && dpkg -i liba53*.deb \
 && git clone https://github.com/RangeNetworks/libcoredumper.git \
 && cd libcoredumper && ./build.sh && dpkg -i *.deb || dpkg -i libcoredumper/*.deb
# openbts master (recursive: CommonLibs + NodeManager) + zeromq helper.
RUN git clone --recursive https://github.com/RangeNetworks/openbts.git openbts
WORKDIR /src/openbts
RUN git checkout ${OPENBTS_REV} \
 && git submodule update --init --recursive \
 && ./NodeManager/install_libzmq.sh || true
RUN ./autogen.sh \
 && ./configure \
 && make -j$(nproc) \
 && make install \
 && ldconfig
# smqueue is NOT part of the normal build: standalone makefile (see README).
RUN make -f Makefile.standalone -C smqueue || (cd smqueue && make -f Makefile.standalone) \
 && cp smqueue/smqueue /usr/local/sbin/smqueue || cp apps/smqueue /usr/local/sbin/smqueue || true
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
RUN sed -i 's|http://archive.ubuntu.com|http://old-releases.ubuntu.com|g; s|http://security.ubuntu.com|http://old-releases.ubuntu.com|g' /etc/apt/sources.list \
 && apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 libortp9 libusb-1.0-0 libreadline6 libzmq5 \
    libboost-date-time1.58.0 libboost-filesystem1.58.0 libboost-regex1.58.0 \
    libboost-system1.58.0 libboost-thread1.58.0 libboost-serialization1.58.0 \
    asterisk sqlite3 iptables iproute2 usbutils curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*
# OpenBTS runtime: binaries, daemons, FPGA images, example SQL seeds.
COPY --from=uhd-builder /usr/local /usr/local
COPY --from=uhd-builder /usr/share/uhd/images /usr/share/uhd/images
COPY --from=openbts-builder /OpenBTS/OpenBTS /OpenBTS/transceiver /OpenBTS/OpenBTSCLI /OpenBTS/OpenBTSDo /OpenBTS/
COPY --from=openbts-builder /usr/local/sbin/sipauthserve /usr/local/sbin/smqueue /usr/local/sbin/comp128 /usr/local/sbin/
COPY --from=openbts-builder /usr/local/lib/liba53.so* /usr/local/lib/libcoredumper.so* /usr/local/lib/
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
    UHD_IMAGES_DIR=/usr/share/uhd/images

EXPOSE 8082
ENTRYPOINT ["/entrypoint.sh"]
