#!/bin/sh
# Build, start, and verify on the SDR server. 在 SDR 服务器构建、启动并验证。
set -eu

COMPOSE_FILE=${COMPOSE_FILE:-deploy/docker/docker-compose.yml}
PROJECT_NAME=${COMPOSE_PROJECT_NAME:-gsm-system-live}
HEALTH_URL=${HEALTH_URL:-http://127.0.0.1:8082/api/v1/cell}
HEALTH_RETRIES=${HEALTH_RETRIES:-30}
ENV_FILE=${GSM_ENV_FILE:-.env}
BUILD=1
VERIFY=1
EXPOSE_API=0

usage() {
  cat <<'EOF'
Usage: ./scripts/deploy_to_ubuntu.sh [options]
  --compose-file PATH  Compose file (default: deploy/docker/docker-compose.yml)
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

# Pass the repository-root environment file to Compose without sourcing it in
# this shell. This keeps optional secrets out of process output and makes the
# deployment independent of Compose's implicit .env lookup rules.
# 将项目根环境文件直接交给 Compose，不在当前 shell 中 source/eval。
compose() {
  if [ "$EXPOSE_API" = 1 ]; then
    set -- -p "$PROJECT_NAME" -f "$COMPOSE_FILE" -f deploy/docker/docker-compose.api.yml "$@"
  else
    set -- -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
  fi
  if [ -f "$ENV_FILE" ]; then
    docker compose --env-file "$ENV_FILE" "$@"
  else
    docker compose "$@"
  fi
}

VERSION=${GSM_VERSION:-$(sed -n '1p' VERSION | tr -d '[:space:]')}
printf '%s\n' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || {
  echo "VERSION must be semantic x.y.z: $VERSION" >&2
  exit 2
}

REVISION=${GSM_REVISION:-}
if [ -z "$REVISION" ] && [ -f .release-revision ]; then
  REVISION=$(sed -n '1p' .release-revision | tr -d '[:space:]')
fi
if [ -z "$REVISION" ] && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  REVISION=$(git rev-parse --short=12 HEAD)
fi
case "$REVISION" in
  ''|*[!0-9a-f]*) echo "revision must be 12 lowercase hexadecimal characters: $REVISION" >&2; exit 2 ;;
esac
[ "${#REVISION}" -eq 12 ] || { echo "revision must contain 12 characters: $REVISION" >&2; exit 2; }

RUNTIME_IMAGE=gsm-system:2.1
if [ "${GSM_IMAGE+x}" = x ] && [ "$GSM_IMAGE" != "$RUNTIME_IMAGE" ]; then
  echo "GSM_IMAGE must remain $RUNTIME_IMAGE: $GSM_IMAGE" >&2
  exit 2
fi
GSM_IMAGE=$RUNTIME_IMAGE
GSM_VERSION=$VERSION
GSM_REVISION=$REVISION
export GSM_IMAGE GSM_VERSION GSM_REVISION

# Ask Compose for its resolved interpolation inputs so .env and shell precedence
# agree with the actual mount. Never source/eval or print the environment (Token).
# 从 Compose 读取实际插值输入，确保 .env 卷名与预检一致；不输出环境或令牌。
compose_environment=$(compose config --environment)
DATA_VOLUME=$(printf '%s\n' "$compose_environment" | sed -n 's/^GSM_DATA_VOLUME=//p')
expose_api=$(printf '%s\n' "$compose_environment" | sed -n 's/^GSM_EXPOSE_API=//p')
case "${expose_api:-false}" in
  true|1) EXPOSE_API=1 ;;
  false|0) EXPOSE_API=0 ;;
  *) echo 'GSM_EXPOSE_API must be true/false or 1/0' >&2; exit 2 ;;
esac
if [ "$EXPOSE_API" = 1 ]; then
  [ -f deploy/docker/docker-compose.api.yml ] || { echo 'API Compose overlay missing' >&2; exit 2; }
fi
unset compose_environment
DATA_VOLUME=${DATA_VOLUME:-docker_gsm-data}
case "$DATA_VOLUME" in
  [A-Za-z0-9]* ) ;;
  *) echo 'invalid data volume name' >&2; exit 2 ;;
esac
case "$DATA_VOLUME" in *[!A-Za-z0-9_.-]*) echo 'invalid data volume name' >&2; exit 2 ;; esac
docker volume inspect "$DATA_VOLUME" >/dev/null

if [ "$BUILD" -eq 1 ]; then
  compose build
fi

verify_image_contract() {
  label_version=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.version" }}' "$GSM_IMAGE") || return 1
  label_revision=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "$GSM_IMAGE") || return 1
  [ "$label_version" = "$VERSION" ] || { echo "image version label mismatch: $label_version" >&2; return 1; }
  [ "$label_revision" = "$REVISION" ] || { echo "image revision label mismatch: $label_revision" >&2; return 1; }
  binary_version=$(docker run --rm --entrypoint /usr/local/bin/gsm-system "$GSM_IMAGE" --version) || return 1
  [ "$binary_version" = "gsm-system $VERSION ($REVISION)" ] || {
    echo "image binary version mismatch: $binary_version" >&2
    return 1
  }
}
verify_image_contract

# Capture intended image IDs before starting; an unrelated API on the host port
# must not turn an exited/wrong-image deployment into a false success.
# 启动前锁定预期镜像 ID，避免宿主机上其它 API 的 200 响应掩盖失败部署。
images=$(compose config --images)
[ -n "$images" ] || { echo 'compose has no images to verify' >&2; exit 1; }
services=$(compose config --services)
[ -n "$services" ] || { echo 'compose has no services to verify' >&2; exit 1; }
expected_ids=''
for image in $images; do
  image_id=$(docker image inspect --format '{{.Id}}' "$image")
  expected_ids="$expected_ids $image_id"
done

compose up -d

HEALTH_CONTAINER=''
verify_containers() {
  # Query only services declared by the current Compose file. `ps` without a
  # service also returns retained same-project orphans used for rollback.
  # 仅检查当前配置声明的服务；同项目保留的回滚 orphan 不参与本次发布验收。
  for service in $services; do
    ids=$(compose ps --all -q "$service") || return 1
    [ -n "$ids" ] || { echo "no deployment container found for service: $service" >&2; return 1; }
    for id in $ids; do
      running=$(docker inspect --format '{{.State.Running}}' "$id") || return 1
      actual_image=$(docker inspect --format '{{.Image}}' "$id") || return 1
      if [ "$running" != true ]; then
        echo "deployment container is not running: $service $id" >&2
        return 1
      fi
      case " $expected_ids " in
        *" $actual_image "*) ;;
        *) echo "unexpected deployment image: $service $id $actual_image" >&2; return 1 ;;
      esac
      if [ -z "$HEALTH_CONTAINER" ] || [ "$service" = gsm-system ]; then
        HEALTH_CONTAINER=$id
      fi
    done
  done
}
verify_containers

if [ "$VERIFY" -eq 1 ]; then
  attempt=1
  health_check() {
    # Read the effective token only inside the newly started container. A token
    # present solely in root .env therefore works without export/source/eval.
    # Feed the header over stdin so the value is absent from both Docker CLI and
    # container curl argv; neither the script nor curl prints the request header.
    # 仅在新容器内读取实际令牌；经 stdin 传递请求头，不进入 Docker/curl argv 或日志。
    docker exec "$HEALTH_CONTAINER" sh -c '
      url=$1
      if [ -n "${GSM_API_TOKEN:-}" ]; then
        printf "Authorization: Bearer %s\n" "$GSM_API_TOKEN" |
          curl -fsS --max-time 30 -H @- "$url"
        exit $?
      fi
      exec curl -fsS --max-time 30 "$url"
    ' sh "$HEALTH_URL"
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
  # HTTP 200 from /cell is not sufficient: the binary probe also rejects a
  # transitioning/degraded running cell while accepting an intentionally
  # stopped cell. /cell 的 200 不代表进程健康，清理前必须通过二进制探针。
  docker exec "$HEALTH_CONTAINER" /usr/local/bin/gsm-system --healthcheck
fi
verify_containers

if [ "$VERIFY" -eq 1 ]; then
  # Only a fully validated deployment may remove superseded GSM runtime
  # objects. Exact image metadata and Compose/managed labels keep container
  # and image cleanup away from LTE and unrelated workloads. Build cache has
  # no repository namespace, so only Docker's unused cache is pruned; images,
  # containers, networks and volumes are never passed to a system prune.
  # 仅在完整验收后清理旧 GSM 对象；数据卷、LTE 镜像及容器均不参与清理。
  current_image_id=$(docker image inspect --format '{{.Id}}' "$GSM_IMAGE")

  managed_containers=$(docker ps -aq --filter 'label=com.gsm-system.managed=true')
  for id in $managed_containers; do
    case " $HEALTH_CONTAINER " in *" $id "*) continue ;; esac
    running=$(docker inspect --format '{{.State.Running}}' "$id")
    [ "$running" = true ] || docker rm "$id" >/dev/null
  done

  gsm_image_ids=$(docker image ls -aq --no-trunc \
    --filter 'label=org.opencontainers.image.title=gsm-system' \
    | sort -u)
  for image_id in $gsm_image_ids; do
    if [ "$image_id" != "$current_image_id" ]; then
      containers=$(docker ps -aq --filter "ancestor=$image_id")
      for id in $containers; do
        running=$(docker inspect --format '{{.State.Running}}' "$id")
        [ "$running" = true ] || docker rm "$id" >/dev/null
      done
      [ -z "$(docker ps -aq --filter "ancestor=$image_id")" ] || continue
    fi

    # Untag only this repository. If an image ID is shared with another
    # repository, that unrelated tag and image remain intact.
    # 仅删除 gsm-system 仓库 tag；共享同一 ID 的其他仓库不受影响。
    repo_tags=$(docker image inspect --format '{{range .RepoTags}}{{println .}}{{end}}' "$image_id")
    for repo_tag in $repo_tags; do
      case "$repo_tag" in
        gsm-system:2.1) [ "$image_id" = "$current_image_id" ] && continue ;;
        gsm-system:*) docker image rm "$repo_tag" >/dev/null ;;
      esac
    done

    # A superseded dangling GSM image has no repository tag to remove. Delete
    # it by ID only when no foreign tag remains.
    if [ "$image_id" != "$current_image_id" ] && \
       docker image inspect "$image_id" >/dev/null 2>&1; then
      repo_tags=$(docker image inspect --format '{{range .RepoTags}}{{println .}}{{end}}' "$image_id")
      [ -n "$repo_tags" ] || docker image rm "$image_id" >/dev/null
    fi
  done
  docker builder prune --all --force >/dev/null
  printf 'Validated %s at revision %s; obsolete GSM objects and unused build cache cleaned / 已验收并清理\n' \
    "$GSM_IMAGE" "$REVISION"
fi
