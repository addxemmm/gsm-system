#!/bin/sh
# prefetch_vendor.sh — runs ON the SDR server (host git works; xenial TLS does not).
# Fills ~/gsm-system/third_party/ with exact-rev sources for the hermetic Dockerfile.
# (Named third_party, NOT vendor: Go auto-enables vendor mode on ./vendor/ and
# would break the go-builder stage.)
# Idempotent: skips trees that already exist. ~0.5GB, untracked, never synced.
# Usage: ./scripts/prefetch_vendor.sh   (on vm-sdr, in ~/gsm-system)
set -eu
cd "$(dirname "$0")/.."
mkdir -p third_party
[ -d third_party/uhd ] || git clone --depth 1 --branch release_003_009_000 --recursive \
  https://github.com/EttusResearch/uhd.git third_party/uhd
# folk/uhd4: UHD 4.1.0.0 host-only (no FPGA submodule needed; .bin ships in
# firmware/uhd/). Paired with the BlackSDR-mini clone image on this board.
[ -d third_party/uhd4 ] || git clone --depth 1 --branch v4.1.0.0 \
  https://github.com/EttusResearch/uhd.git third_party/uhd4
# folk/uhd4: cppzmq single header (no 22.04 package; zmq.hpp split from
# libzmq in 4.2+). v4.7.x is era-correct for libzmq 4.3.x.
[ -d third_party/cppzmq ] || git clone --depth 1 --branch v4.7.1 \
  https://github.com/zeromq/cppzmq.git third_party/cppzmq
[ -d third_party/openbts ] || git clone --recursive \
  https://github.com/RangeNetworks/openbts.git third_party/openbts
# smqueue + subscriberRegistry (sipauthserve) both left the openbts tree upstream.
[ -d third_party/smqueue ] || git clone --recursive \
  https://github.com/RangeNetworks/smqueue.git third_party/smqueue
[ -d third_party/subscriberRegistry ] || git clone --recursive \
  https://github.com/RangeNetworks/subscriberRegistry.git third_party/subscriberRegistry
[ -d third_party/liba53 ] || git clone \
  https://github.com/RangeNetworks/liba53.git third_party/liba53
[ -d third_party/libcoredumper ] || git clone \
  https://github.com/RangeNetworks/libcoredumper.git third_party/libcoredumper
du -sh third_party
