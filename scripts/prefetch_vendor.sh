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
[ -d third_party/openbts ] || git clone --recursive \
  https://github.com/RangeNetworks/openbts.git third_party/openbts
[ -d third_party/liba53 ] || git clone \
  https://github.com/RangeNetworks/liba53.git third_party/liba53
[ -d third_party/libcoredumper ] || git clone \
  https://github.com/RangeNetworks/libcoredumper.git third_party/libcoredumper
[ -d third_party/libzmq ] || git clone --depth 1 --branch v4.3.4 \
  https://github.com/zeromq/libzmq.git third_party/libzmq
du -sh third_party
