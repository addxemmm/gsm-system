#!/bin/sh
# prefetch_vendor.sh — runs ON the SDR server (host git works; xenial TLS does not).
# Fills ~/gsm-system/vendor/ with exact-rev sources for the hermetic Dockerfile.
# Idempotent: skips trees that already exist. ~0.5GB, untracked, never synced.
# Usage: ./scripts/prefetch_vendor.sh   (on vm-sdr, in ~/gsm-system)
set -eu
cd "$(dirname "$0")/.."
mkdir -p vendor
[ -d vendor/uhd ] || git clone --depth 1 --branch release_003_009_000 --recursive \
  https://github.com/EttusResearch/uhd.git vendor/uhd
[ -d vendor/openbts ] || git clone --recursive \
  https://github.com/RangeNetworks/openbts.git vendor/openbts
[ -d vendor/liba53 ] || git clone \
  https://github.com/RangeNetworks/liba53.git vendor/liba53
[ -d vendor/libcoredumper ] || git clone \
  https://github.com/RangeNetworks/libcoredumper.git vendor/libcoredumper
[ -d vendor/libzmq ] || git clone --depth 1 --branch v4.3.4 \
  https://github.com/zeromq/libzmq.git vendor/libzmq
du -sh vendor
