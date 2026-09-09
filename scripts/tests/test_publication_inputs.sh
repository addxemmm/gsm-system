#!/bin/sh
# Offline input gate: no Docker, network, services or subscriber values.
# 离线输入门禁：不运行 Docker、网络、服务，不打印用户数据。
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
go run ./scripts/release/publication
go test ./scripts/release/publication
