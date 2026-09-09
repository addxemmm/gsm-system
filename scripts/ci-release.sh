#!/bin/sh
# Read-only Docker Hub inspection; Go stdlib, no Python or credential logging.
# Docker Hub 只读镜像校验；Go 标准库，不依赖 Python、不记录凭据。
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"
exec go run ./scripts/release/registry "$@"
