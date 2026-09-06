#!/bin/sh
# Build, start, and verify on the SDR server. 在 SDR 服务器构建、启动并验证。
set -eu

COMPOSE_FILE=${COMPOSE_FILE:-deploy/docker/docker-compose.uhd4.yml}
PROJECT_NAME=${COMPOSE_PROJECT_NAME:-gsm-system-live}
HEALTH_URL=${HEALTH_URL:-http://127.0.0.1:8082/api/v1/cell}
HEALTH_RETRIES=${HEALTH_RETRIES:-30}
BUILD=1
VERIFY=1

usage() {
  cat <<'EOF'
Usage: ./scripts/deploy_to_ubuntu.sh [options]
  --compose-file PATH  Compose file (default: deploy/docker/docker-compose.uhd4.yml)
  --project-name NAME  Compose project (default: gsm-system-live)
  --skip-build         Start the already-built image
  --skip-health        Do not poll the HTTP health endpoint
  --health-url URL     Health endpoint (default: http://127.0.0.1:8082/api/v1/cell)
  -h, --help           Show help

This script runs `compose up -d`; use deploy_from_windows.ps1 for sync/build only.
本脚本会执行 `compose up -d`；仅同步或构建请使用 deploy_from_windows.ps1。
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --compose-file)
      [ "$#" -ge 2 ] || { echo "missing value for --compose-file" >&2; exit 2; }
      COMPOSE_FILE=$2
      shift 2
      ;;
    --skip-build) BUILD=0; shift ;;
    --project-name)
      [ "$#" -ge 2 ] || { echo "missing value for --project-name" >&2; exit 2; }
      PROJECT_NAME=$2
      shift 2
      ;;
    --skip-health) VERIFY=0; shift ;;
    --health-url)
      [ "$#" -ge 2 ] || { echo "missing value for --health-url" >&2; exit 2; }
      HEALTH_URL=$2
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

case "$COMPOSE_FILE" in
  /*|*../*|*/..|..) echo "compose path must be repository-relative: $COMPOSE_FILE" >&2; exit 2 ;;
esac
case "$PROJECT_NAME" in
  ''|[!a-z0-9]*|*[!a-z0-9_-]*) echo "invalid compose project name: $PROJECT_NAME" >&2; exit 2 ;;
esac
case "$HEALTH_RETRIES" in
  ''|*[!0-9]*) echo "HEALTH_RETRIES must be a positive integer" >&2; exit 2 ;;
esac
[ "$HEALTH_RETRIES" -gt 0 ] || { echo "HEALTH_RETRIES must be greater than zero" >&2; exit 2; }

cd "$(dirname "$0")/.."
[ -f "$COMPOSE_FILE" ] || { echo "compose file not found: $COMPOSE_FILE" >&2; exit 2; }

if [ "$BUILD" -eq 1 ]; then
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" build
fi

# Capture intended image IDs before starting; an unrelated API on the host port
# must not turn an exited/wrong-image deployment into a false success.
# 启动前锁定预期镜像 ID，避免宿主机上其它 API 的 200 响应掩盖失败部署。
images=$(docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" config --images)
[ -n "$images" ] || { echo 'compose has no images to verify' >&2; exit 1; }
expected_ids=''
for image in $images; do
  image_id=$(docker image inspect --format '{{.Id}}' "$image")
  expected_ids="$expected_ids $image_id"
done

docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d

verify_containers() {
  ids=$(docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" ps --all -q) || return 1
  [ -n "$ids" ] || { echo 'no deployment containers found' >&2; return 1; }
  for id in $ids; do
    running=$(docker inspect --format '{{.State.Running}}' "$id") || return 1
    actual_image=$(docker inspect --format '{{.Image}}' "$id") || return 1
    if [ "$running" != true ]; then
      echo "deployment container is not running: $id" >&2
      return 1
    fi
    case " $expected_ids " in
      *" $actual_image "*) ;;
      *) echo "unexpected deployment image: $id $actual_image" >&2; return 1 ;;
    esac
  done
}
verify_containers

if [ "$VERIFY" -eq 1 ]; then
  attempt=1
  health_check() {
    if [ -n "${GSM_API_TOKEN:-}" ]; then
      curl -fsS --max-time 30 -H "Authorization: Bearer $GSM_API_TOKEN" "$HEALTH_URL"
    else
      curl -fsS --max-time 30 "$HEALTH_URL"
    fi
  }
  while ! health_check; do
    if [ "$attempt" -ge "$HEALTH_RETRIES" ]; then
      echo "health check failed after $attempt attempts: $HEALTH_URL" >&2
      exit 1
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  echo
fi
verify_containers
