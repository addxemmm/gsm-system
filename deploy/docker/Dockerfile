# syntax=docker/dockerfile:1
# gsm-system 2.1: OpenBTS 5.0 + UHD 4.1.0.0 on Ubuntu 22.04.
# Why: the GSM-VM board is an Artix-7 BlackSDR mini clone; only the 4.x-era
# clone FPGA image configures it, so the radio stack must move to UHD 4.x.
# Upstream OpenBTS caps at UHD 3.x (uhd::msg was removed in 4.0); the port
# surface is one file (Transceiver52M/UHDDevice.cpp, ~25 symbols) + the
# compat/uhd/utils/msg.hpp shim in this repo. Timeboxed: 2 rounds, then stop.
#
# Base is Ubuntu 22.04 (gcc-11 builds UHD 4.1; xenial gcc-5 cannot).
# 2019-era C++ + gcc-11 + upstream -Werror does not mix, so user code builds
# with -w (see below); warnings from a decade-old tree on a new compiler are
# noise, not signal. Native sources come from the verified server-side
# third_party/ cache (see scripts/prefetch_vendor.sh); builders never clone.
#
# Stages:
#   uhd4-builder     -> UHD v4.1.0.0 (host-only, no FPGA build needed)
#   openbts4-builder -> liba53 + libcoredumper + openbts + smqueue + subscriberRegistry
#   go-builder       -> Go single binary (same as main line)
#   runtime          -> clone FPGA image + asterisk + seeds. NO python env needed
#                      by the management plane (apt may pull Python transitively).

ARG VERSION=2.1.0
ARG REVISION=unknown

# Fail before the expensive native build if the host cache is not the pinned,
# prefetch-verified source set. The manifest is also shipped for provenance.
FROM ubuntu:22.04 AS source-manifest
COPY third_party/REVISION_MANIFEST.tsv /verified/REVISION_MANIFEST.tsv
RUN set -eu; \
    check_revision() { \
      count=$(awk -F '\t' -v component="$1" -v revision="$2" \
        '$1 == component && $2 == "repository" && $3 == revision && $4 == revision { count++ } END { print count + 0 }' \
        /verified/REVISION_MANIFEST.tsv); \
      [ "$count" -eq 1 ] || { echo "missing or duplicate pinned revision: $1 $2" >&2; exit 1; }; \
    }; \
    check_revision cppzmq 76bf169fd67b8e99c1b0e6490029d9cd5ef97666; \
    check_revision liba53 27354560dc7b554e03d40a520d41290e731193b6; \
    check_revision libcoredumper 7527fb3804927c7fdc72ff5139a2cdea3db4d59a; \
    check_revision openbts 7766ef94f2d885c197430e74a89f02740f0c04e6; \
    check_revision smqueue e168a262db311231c51cf7295f9bd1f440567485; \
    check_revision subscriberRegistry c65b5d59f744a8df5f3395e217f2598f53e9fe65; \
    check_revision uhd4 d21735d543d5a3c265507965c7bb6c9e9df95fcd

# ---------- stage 1: UHD 4.1.0.0 ----------
FROM ubuntu:22.04 AS uhd4-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake pkg-config python3 python3-mako \
    libboost-all-dev libusb-1.0-0-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY third_party/uhd4 /src/uhd
COPY --from=source-manifest /verified/REVISION_MANIFEST.tsv /verified/REVISION_MANIFEST.tsv
# python3/mako above are upstream UHD build tools, not management runtime code.
WORKDIR /src/uhd/host/build
RUN cmake ../ -DCMAKE_BUILD_TYPE=Release -DENABLE_TESTS=OFF -DENABLE_EXAMPLES=OFF -DENABLE_UTILS=ON \
 && make -j$(nproc) \
 && make install \
 && ldconfig

# ---------- stage 2: OpenBTS family on gcc-11 ----------
FROM ubuntu:22.04 AS openbts4-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential autoconf automake libtool pkg-config dpkg-dev debhelper git \
    sqlite3 libsqlite3-dev libortp-dev libusb-1.0-0-dev libreadline-dev \
    libboost-all-dev libzmq3-dev libosip2-dev ca-certificates \
 && rm -rf /var/lib/apt/lists/*
# cppzmq.hpp has no 22.04 package: vendor the single header (era: v4.7.x).
COPY third_party/cppzmq/zmq.hpp third_party/cppzmq/zmq_addon.hpp /usr/include/
COPY --from=uhd4-builder /usr/local /usr/local
RUN ldconfig
# msg shim for uhd::msg (removed in UHD 4.0); -I order puts it first.
# -std=gnu++14: era-correct standard (2019 code + gcc-11 defaults to C++17,
# where dynamic exception specs and `register` are hard errors, not warnings).
# -fpermissive: era int<->pointer conversions (e.g. dialog IDs as void*).
# -k on make: surface ALL translation-unit errors per run, not just the first.
# -DORTP_NEW_API=1: configure's compile-test mis-detects ortp5 as <0.22.0;
# the 4-arg rtp_session_set_local_addr is verified present in
# /usr/include/ortp/rtpsession.h, and upstream code already handles it.
COPY compat /app/compat
ENV CXXFLAGS="-g -O2 -w -std=gnu++14 -fpermissive -DORTP_NEW_API=1 -I/app/compat"
WORKDIR /src
COPY third_party/liba53 ./liba53
COPY third_party/libcoredumper ./libcoredumper
# liba53 via plain make (its debian/compat predates 22.04 debhelper).
# NOTE: its Makefile installs to /usr/lib (hardcoded DESTDIR prefix).
RUN cd liba53 && make -j$(nproc) && make install && ldconfig && ls /usr/lib/liba53*
# libcoredumper: same steps as its build.sh, plus compat bump for new debhelper.
# libcoredumper: plain configure+make+install to /usr/local (its 2007
# debian/ dir is uninstallable on modern debhelper: compat level rejected,
# multiarch paths mismatch). Same installed result the .deb used to provide.
RUN cd libcoredumper \
 && test -f coredumper-1.2.1.tar.gz \
 && test "$(wc -c < coredumper-1.2.1.tar.gz)" -eq 437668 \
 && printf '%s  %s\n' c7dc2f0ee2b00a3192bd1fa595d175ce6ecf6c5499220ba1232fa2fb4cc3774d coredumper-1.2.1.tar.gz | sha256sum -c - \
 && rm -rf coredumper-1.2.1 && tar zxf coredumper-1.2.1.tar.gz && cd coredumper-1.2.1 \
 && sed -i '/#include <sys\/sysctl.h>/d' src/elfcore.c \
 && patch -p0 < ../fix_from_scratch_build.patch \
 && ./configure && make -j$(nproc) && make install && ldconfig \
 && ls /usr/local/lib/libcoredumper*
COPY third_party/openbts ./openbts
# Upstream compat patches for new toolchains (see compat/patches/).
RUN cd openbts \
 && patch -p1 < /app/compat/patches/0001-commonlibs-gettid-glibc230.patch \
 && patch -p1 < /app/compat/patches/0002-sip-ortp-bctbx-loghook.patch \
 && patch -p1 < /app/compat/patches/0004-gmm-optional-tv-ies.patch \
 && sh /app/compat/tests/test-gmm-optional-ies.sh SGSNGGSN/GPRSL3Messages.cpp \
 && patch -p1 < /app/compat/patches/0005-uhd-rx-timeout-retry.patch \
 && sh /app/compat/tests/test-uhd-rx-timeout.sh Transceiver52M/UHDDevice.cpp \
 && sed -i 's/<<os <</<<os.str()<</g' Control/L3StateMachine.cpp \
 && sed -i 's/<<this<<os;/<<this<<os.str();/' GPRS/MSInfo.cpp \
 && sed -i 's/"<<ss)/"<<ss.str())/g' SGSNGGSN/Sgsn.cpp
WORKDIR /src/openbts
RUN ./autogen.sh \
 && ./configure --with-uhd \
 && make -j$(nproc) -k \
 && make install \
 && ldconfig
COPY third_party/smqueue ./smqueue
RUN apt-get update && apt-get install -y --no-install-recommends libosip2-dev && rm -rf /var/lib/apt/lists/* \
 && cd smqueue \
 && patch -p1 < /app/compat/patches/0003-smqueue-keyed-sms-observation.patch \
 && patch -p1 < /app/compat/patches/0006-smqueue-ucs2-decode.patch \
 && sh /app/compat/tests/test-sms-ucs2.sh SMS/SMSMessages.cpp \
 && grep -rl '!defined(gettid)' CommonLibs SR/CommonLibs 2>/dev/null | xargs -r sed -i 's/#if !defined(gettid)/#if !defined(gettid) \&\& !__GLIBC_PREREQ(2,30)/' \
 && ./autogen.sh && ./configure && make -j$(nproc) -k && make install && ldconfig
COPY third_party/subscriberRegistry ./subscriberRegistry
RUN cd subscriberRegistry \
 && grep -rl '!defined(gettid)' CommonLibs 2>/dev/null | xargs -r sed -i 's/#if !defined(gettid)/#if !defined(gettid) \&\& !__GLIBC_PREREQ(2,30)/' \
 && ./autogen.sh && ./configure && make -j$(nproc) -k && make install && ldconfig
# Normalize Debian-packaged libs for the runtime COPY + fail loudly if absent.
RUN for f in /usr/lib/liba53* /usr/lib/libcoredumper* /usr/lib/x86_64-linux-gnu/liba53* /usr/lib/x86_64-linux-gnu/libcoredumper*; do [ -e "$f" ] && cp -P "$f" /usr/local/lib/; done; ldconfig; ls /usr/local/lib/liba53* /usr/local/lib/libcoredumper*
RUN ls -l /OpenBTS/OpenBTS /OpenBTS/transceiver /usr/local/sbin/sipauthserve /usr/local/sbin/smqueue

# ---------- stage 3: Go API server (same as main line) ----------
FROM golang:1.26.8-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
ARG VERSION
ARG REVISION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.revision=${REVISION}" \
    -o /out/gsm-system ./cmd/server

# ---------- stage 4: runtime ----------
FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 libortp15 libusb-1.0-0 libreadline8 libzmq5 \
    libboost-date-time1.74.0 libboost-filesystem1.74.0 libboost-regex1.74.0 \
    libboost-system1.74.0 libboost-thread1.74.0 libboost-serialization1.74.0 \
    libboost-program-options1.74.0 libboost-chrono1.74.0 \
    libosip2-11 libsqliteodbc \
    asterisk sqlite3 iptables iproute2 usbutils curl ca-certificates procps psmisc tzdata \
 && rm -rf /var/lib/apt/lists/*
COPY --from=openbts4-builder /usr/local /usr/local
# Clone-board FPGA image (Artix-7) + B2xx FX3 firmware, paired with UHD 4.1.
# Provenance: FPGA from vendor bundle (firmware/uhd/README.md); fw/bl from
# the 4.x images set (verified on-wire: loopback passed on serial 2548123).
COPY firmware/uhd/usrp_b210_fpga.blacksdr.bin /usr/share/uhd/images/usrp_b210_fpga.bin
COPY firmware/uhd/usrp_b200_fw.hex firmware/uhd/usrp_b200_bl.img /usr/share/uhd/images/
COPY --from=openbts4-builder /OpenBTS/OpenBTS /OpenBTS/transceiver /OpenBTS/OpenBTSCLI /OpenBTS/OpenBTSDo /OpenBTS/
COPY --from=openbts4-builder /usr/local/sbin/sipauthserve /usr/local/sbin/smqueue /usr/local/sbin/comp128 /usr/local/sbin/
RUN ldconfig \
 && ln -sf /usr/local/sbin/sipauthserve /OpenBTS/sipauthserve \
 && ln -sf /usr/local/sbin/smqueue /OpenBTS/smqueue \
 && ldd /OpenBTS/transceiver | grep -q libuhd \
 && ldd /OpenBTS/OpenBTS | grep -c "not found" | grep -q "^0$"
COPY gsmsystem/asterisk/ /etc/asterisk/
COPY deploy/docker/sip-contact-acl.conf /etc/asterisk/sip-contact-acl.conf
# Asterisk's default contact ACL rejects loopback. smqueue registers the local
# OpenBTS contact after assigning a number; permit only that address, not all LANs.
# 默认联系人 ACL 拒绝回环；只放行同容器 OpenBTS，保留外部地址拒绝规则。
RUN sed -i '/^\[general\][[:space:]]*$/a #include sip-contact-acl.conf' /etc/asterisk/sip.conf \
 && grep -Fxq '#include sip-contact-acl.conf' /etc/asterisk/sip.conf
# The archived v1 modules.conf disables ODBC realtime; enable it in the image
# without editing the read-only legacy archive. SIP users require this module.
# 归档配置禁用了 ODBC realtime；在镜像层启用，保留旧档案和本地未提交改动。
RUN sed -i '/^[[:space:]]*noload[[:space:]]*=>[[:space:]]*res_config_odbc\.so/s/^/;/' /etc/asterisk/modules.conf
COPY deploy/docker/odbc.ini /etc/odbc.ini
COPY deploy/docker/odbcinst.ini /etc/odbcinst.ini
COPY configs/seeds/TMSITable_init.db configs/seeds/sqlite3_init.db /OpenBTS/
COPY --from=openbts4-builder /src/openbts/apps/OpenBTS.example.sql /app/seeds/OpenBTS.example.sql
COPY --from=go-builder /out/gsm-system /usr/local/bin/gsm-system
COPY --from=source-manifest /verified/REVISION_MANIFEST.tsv /usr/share/doc/gsm-system/VENDOR-REVISIONS.tsv
COPY configs/ /app/configs/
RUN cp /app/configs/app.yaml.example /app/configs/app.yaml \
 && mkdir -p /data/conf /data/log /etc/OpenBTS /var/run /var/lib/asterisk/sqlite3dir
COPY deploy/docker/entrypoint.sh /entrypoint.sh
COPY deploy/docker/persistent-state.sh /usr/local/lib/gsm-persistent-state.sh
COPY deploy/docker/timezone.sh /usr/local/lib/gsm-timezone.sh
RUN chmod +x /entrypoint.sh /OpenBTS/OpenBTSDo

# Revision metadata follows expensive runtime layers so API-only releases reuse them.
# 将版本元数据放在运行依赖之后，API 迭代不重复安装系统包。
ARG VERSION
ARG REVISION
LABEL org.opencontainers.image.title="gsm-system" \
      org.opencontainers.image.description="OpenBTS 5 with UHD 4 and Go management API" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.source="https://github.com/addxemmm/gsm-system"

ENV GSM_CONFIG=/app/configs/app.yaml \
    GSM_DATA_DIR=/data \
    TZ=Asia/Shanghai \
    UHD_IMAGES_DIR=/usr/share/uhd/images

EXPOSE 8082
ENTRYPOINT ["/entrypoint.sh"]
