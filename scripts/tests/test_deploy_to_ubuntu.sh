#!/bin/sh
# Offline contract tests: fake docker/curl/sleep, no network or daemon.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
TMP=${TMPDIR:-/tmp}/gsm-deploy-test-$$
BIN=$TMP/bin
LOG=$TMP/native.log
REVISION=$(git -C "$ROOT" rev-parse --short=12 HEAD)
EXPECTED_VERSION=$(tr -d '\r\n' <"$ROOT/VERSION")
IMAGE=gsm-system:2.1
mkdir -p "$BIN"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT HUP INT TERM

cat >"$BIN/docker" <<'EOF'
#!/bin/sh
printf 'docker image=%s version=%s revision=%s %s\n' \
  "${GSM_IMAGE:-}" "${GSM_VERSION:-}" "${GSM_REVISION:-}" "$*" >>"$FAKE_DEPLOY_LOG"
case " $* " in
  *" compose "*" config --environment "*)
    printf 'GSM_DATA_VOLUME=%s\nGSM_API_TOKEN=fixture-secret-not-printed\n' "${FAKE_COMPOSE_VOLUME:-${GSM_DATA_VOLUME:-docker_gsm-data}}"
    printf 'GSM_EXPOSE_API=%s\n' "${FAKE_COMPOSE_EXPOSE_API:-false}"
    ;;
  *" compose "*" build "*) [ "${FAKE_BUILD_FAIL:-0}" != 1 ] ;;
  *" compose "*" config --images "*) echo "$GSM_IMAGE" ;;
  *" compose "*" config --services "*) echo 'gsm-system' ;;
  *" compose "*" ps --all -q gsm-system "*) echo 'fixture-current' ;;
  *" compose "*" ps --all -q "*) printf '%s\n' 'fixture-current' 'rollback-orphan' ;;
  *" volume inspect "*) exit 0 ;;
  *" image inspect "*"org.opencontainers.image.version"*) echo "${FAKE_LABEL_VERSION:-$GSM_VERSION}" ;;
  *" image inspect "*"org.opencontainers.image.revision"*) echo "${FAKE_LABEL_REVISION:-$GSM_REVISION}" ;;
  *" image inspect "*"range .RepoTags"*" sha256:fixture "*) printf '%s\n' 'gsm-system:2.1' 'gsm-system:2.1.0' 'addxemmm/gsm-system:2.1' ;;
  *" image inspect "*"range .RepoTags"*" sha256:old "*) printf '%s\n' 'gsm-system:2.1.0' 'gsm-system:2.1.0-oldrevision' 'addxemmm/gsm-system:2.0' ;;
  *" image inspect "*"range .RepoTags"*" sha256:shared "*) printf '%s\n' 'gsm-system:legacy' 'lte:preserve' ;;
  *" image inspect "*"range .RepoTags"*" sha256:dangling "*) : ;;
  *" image ls -aq --no-trunc "*)
    [ "${FAKE_CLEANUP_FIXTURES:-0}" = 1 ] && printf '%s\n' sha256:fixture sha256:old sha256:shared sha256:dangling || echo sha256:fixture
    ;;
  *" image inspect "*) echo 'sha256:fixture' ;;
  *" run --rm --entrypoint /usr/local/bin/gsm-system "*) echo "${FAKE_BINARY_VERSION:-gsm-system $GSM_VERSION ($GSM_REVISION)}" ;;
  *" exec fixture-current /usr/local/bin/gsm-system --healthcheck "*) [ "${FAKE_BINARY_HEALTH_FAIL:-0}" != 1 ] ;;
  *" exec fixture-current sh -c "*)
    [ "${FAKE_EXEC_FAIL:-0}" != 1 ] || exit 22
    if [ "${FAKE_REQUIRE_CONTAINER_TOKEN:-0}" = 1 ]; then
      [ -n "${FAKE_CONTAINER_TOKEN:-}" ] || exit 22
    fi
    GSM_API_TOKEN=${FAKE_CONTAINER_TOKEN:-}
    export GSM_API_TOKEN
    shift 2
    exec "$@"
    ;;
  *"{{.State.Running}}"*" rollback-orphan "*) echo "${FAKE_ORPHAN_RUNNING:-false}" ;;
  *"{{.State.Running}}"*" fixture-current "*) echo "${FAKE_CURRENT_RUNNING:-true}" ;;
  *"{{.State.Running}}"*" old-stopped "*) echo false ;;
  *"{{.Image}}"*" rollback-orphan "*) echo "${FAKE_ORPHAN_IMAGE:-sha256:rollback}" ;;
  *"{{.Image}}"*" fixture-current "*) echo "${FAKE_CURRENT_IMAGE:-sha256:fixture}" ;;
  *" ps -aq --filter label=com.gsm-system.managed=true "*)
    [ "${FAKE_CLEANUP_FIXTURES:-0}" = 1 ] && [ ! -f "$FAKE_DEPLOY_STATE/old-removed" ] && echo old-stopped || :
    ;;
  *" ps -aq --filter ancestor=sha256:old "*|*" ps -aq --filter ancestor=sha256:shared "*)
    [ ! -f "$FAKE_DEPLOY_STATE/old-removed" ] && echo old-stopped || :
    ;;
  *" ps -aq --filter ancestor="*) : ;;
  *" rm old-stopped "*) mkdir -p "$FAKE_DEPLOY_STATE"; : >"$FAKE_DEPLOY_STATE/old-removed" ;;
  *) exit 0 ;;
esac
EOF
cat >"$BIN/curl" <<'EOF'
#!/bin/sh
if [ "${FAKE_REQUIRE_CONTAINER_TOKEN:-0}" = 1 ]; then
  header=$(cat)
  [ "$header" = "Authorization: Bearer $FAKE_CONTAINER_TOKEN" ] || exit 22
  printf 'curl header=present %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
else
  printf 'curl %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
fi
[ "${FAKE_CURL_FAIL:-0}" != 1 ] || exit 22
printf '{"status":"ok"}'
EOF
cat >"$BIN/sleep" <<'EOF'
#!/bin/sh
printf 'sleep %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
EOF
chmod +x "$BIN/docker" "$BIN/curl" "$BIN/sleep"

# A global Compose ps would expose the stopped rollback orphan and fail. The
# deploy must query only the service declared by the current Compose config.
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state HEALTH_RETRIES=2 \
  FAKE_ORPHAN_RUNNING=false FAKE_ORPHAN_IMAGE=sha256:rollback \
  "$ROOT/scripts/deploy_to_ubuntu.sh"
grep -F "image=$IMAGE version=$EXPECTED_VERSION revision=$REVISION compose -p gsm-system-live -f deploy/docker/docker-compose.yml build" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml config --services" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml ps --all -q gsm-system" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml up -d" "$LOG" >/dev/null
grep -F 'exec fixture-current sh -c' "$LOG" >/dev/null
grep -F 'printf "Authorization: Bearer %s\n" "$GSM_API_TOKEN"' "$LOG" >/dev/null
grep -F 'curl -fsS --max-time 30 -H @-' "$LOG" >/dev/null
grep -F 'http://127.0.0.1:8082/api/v1/cell' "$LOG" >/dev/null
grep -F 'exec fixture-current /usr/local/bin/gsm-system --healthcheck' "$LOG" >/dev/null
grep -F 'builder prune --all --force' "$LOG" >/dev/null
curl_line=$(grep -n 'exec fixture-current sh -c' "$LOG" | tail -1 | cut -d: -f1)
cleanup_line=$(grep -n 'builder prune --all --force' "$LOG" | cut -d: -f1)
[ "$cleanup_line" -gt "$curl_line" ] || { echo 'cleanup ran before HTTP validation' >&2; exit 1; }
if grep -E ' ps --all -q$' "$LOG" >/dev/null || grep -F 'rollback-orphan' "$LOG" >/dev/null; then
  echo 'retained rollback orphan was included in deployment verification' >&2
  exit 1
fi

: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_BUILD_FAIL=1 \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-health; then
  echo "expected failed docker build to fail the deploy" >&2
  exit 1
fi
if grep -F " up -d" "$LOG" >/dev/null; then
  echo "compose up ran after a failed build" >&2
  exit 1
fi

# curl always returns 200 here: it must not hide an exited or stale container.
# 即使假 curl 始终 200，退出容器及旧镜像仍须使部署失败。
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_CURRENT_RUNNING=false \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'exited current-service container incorrectly passed readiness' >&2; exit 1
fi
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_CURRENT_IMAGE=sha256:stale \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'stale current-service image incorrectly passed readiness' >&2; exit 1
fi

: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_EXEC_FAIL=1 HEALTH_RETRIES=1 \
  "$ROOT/scripts/deploy_to_ubuntu.sh"; then
  echo 'failed HTTP readiness incorrectly passed' >&2; exit 1
fi
if grep -F ' builder prune ' "$LOG" >/dev/null; then
  echo 'cleanup ran after failed HTTP readiness' >&2; exit 1
fi

# A token that exists only in the Compose env file must be consumed inside the
# service container, without sourcing the file or exposing its value in logs.
# 仅 Compose 环境文件有令牌时，探针须在容器内取值且不得泄露。
: >"$LOG"
TOKEN_ENV=$TMP/token.env
TOKEN_FIXTURE=fixture-token-do-not-log
printf 'GSM_API_TOKEN=%s\n' "$TOKEN_FIXTURE" >"$TOKEN_ENV"
env -u GSM_API_TOKEN "PATH=$BIN:$PATH" "FAKE_DEPLOY_LOG=$LOG" "FAKE_DEPLOY_STATE=$TMP/state" \
  "GSM_ENV_FILE=$TOKEN_ENV" FAKE_REQUIRE_CONTAINER_TOKEN=1 \
  "FAKE_CONTAINER_TOKEN=$TOKEN_FIXTURE" \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F "compose --env-file $TOKEN_ENV -p gsm-system-live" "$LOG" >/dev/null
grep -F 'printf "Authorization: Bearer %s\n" "$GSM_API_TOKEN"' "$LOG" >/dev/null
if grep -F "$TOKEN_FIXTURE" "$LOG" >/dev/null; then
  echo 'container token leaked into deployment log' >&2; exit 1
fi

# Only the local/Hub 2.1 references are allowed, before any Docker operation.
# 仅支持本地/Hub 的明确 2.1 引用，其余仓库、tag、digest 和空值均提前拒绝。
for rejected_image in '' gsm-system:2.1.0-aaaaaaaaaaaa gsm-system:latest \
  addxemmm/gsm-system:latest addxemmm/gsm-system:2.1.0 other/gsm-system:2.1 \
  docker.io/addxemmm/gsm-system:2.1 'addxemmm/gsm-system@sha256:aaaaaaaaaaaa'; do
  : >"$LOG"
  if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG GSM_IMAGE="$rejected_image" \
    "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build --skip-health; then
    echo "unsupported image override unexpectedly passed: $rejected_image" >&2; exit 1
  fi
  [ ! -s "$LOG" ] || { echo 'unsupported image invoked Docker' >&2; exit 1; }
done

# Hub references must never enter either explicit or implicit build/pull paths.
# Hub 镜像必须预拉取，部署中既不显式构建，也不隐式构建/拉取。
HUB_IMAGE=addxemmm/gsm-system:2.1
: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG GSM_IMAGE=$HUB_IMAGE \
  "$ROOT/scripts/deploy_to_ubuntu.sh"; then
  echo 'Hub image accepted without --skip-build' >&2; exit 1
fi
[ ! -s "$LOG" ] || { echo 'Hub build rejection invoked Docker' >&2; exit 1; }

: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state GSM_IMAGE=$HUB_IMAGE \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build --skip-cleanup
grep -F "image=$HUB_IMAGE version=$EXPECTED_VERSION revision=$REVISION" "$LOG" >/dev/null
grep -F 'up -d --no-build --pull never' "$LOG" >/dev/null
grep -F 'org.opencontainers.image.version' "$LOG" >/dev/null
grep -F 'org.opencontainers.image.revision' "$LOG" >/dev/null
grep -F "run --rm --entrypoint /usr/local/bin/gsm-system $HUB_IMAGE --version" "$LOG" >/dev/null
grep -F 'exec fixture-current sh -c' "$LOG" >/dev/null
grep -F 'exec fixture-current /usr/local/bin/gsm-system --healthcheck' "$LOG" >/dev/null
if grep -E ' build$| builder prune | image rm | image ls -aq' "$LOG" >/dev/null; then
  echo 'Hub skip-build/skip-cleanup performed build or cleanup' >&2; exit 1
fi

# Published images retain the exact same metadata, readiness and health gates.
# Hub 镜像仍须通过元数据、容器状态、HTTP 与二进制健康门禁。
for failure in FAKE_LABEL_VERSION=0.0.0 FAKE_LABEL_REVISION=bbbbbbbbbbbb \
  FAKE_BINARY_VERSION=wrong FAKE_CURRENT_RUNNING=false FAKE_CURRENT_IMAGE=sha256:stale \
  FAKE_EXEC_FAIL=1 FAKE_BINARY_HEALTH_FAIL=1; do
  : >"$LOG"
  if env "PATH=$BIN:$PATH" "FAKE_DEPLOY_LOG=$LOG" "FAKE_DEPLOY_STATE=$TMP/state" \
    "GSM_IMAGE=$HUB_IMAGE" HEALTH_RETRIES=1 "$failure" \
    "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
    echo "Hub image bypassed gate: $failure" >&2; exit 1
  fi
  if grep -E ' builder prune | image rm | image ls -aq' "$LOG" >/dev/null; then
    echo "Hub cleanup ran after failed gate: $failure" >&2; exit 1
  fi
  case "$failure" in
    FAKE_LABEL_*|FAKE_BINARY_VERSION=*)
      if grep -F ' up -d' "$LOG" >/dev/null; then
        echo 'Hub image started before metadata/binary validation' >&2; exit 1
      fi
      ;;
  esac
done

: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state GSM_IMAGE=$HUB_IMAGE \
  FAKE_CLEANUP_FIXTURES=1 "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F 'image rm gsm-system:2.1' "$LOG" >/dev/null
grep -F 'image rm addxemmm/gsm-system:2.0' "$LOG" >/dev/null
if grep -E 'image rm (addxemmm/gsm-system:2\.1|lte:preserve|sha256:fixture)$|volume rm|system prune' "$LOG" >/dev/null; then
  echo 'Hub cleanup removed current image or unrelated objects' >&2; exit 1
fi
echo 'PASS restricted Hub deployment / 受限 Hub 部署契约通过'

echo "PASS test_deploy_to_ubuntu.sh / Ubuntu 部署脚本测试通过"

: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state FAKE_COMPOSE_VOLUME=fixture-env-volume \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F 'volume inspect fixture-env-volume' "$LOG" >/dev/null
if grep -F 'fixture-secret-not-printed' "$LOG" >/dev/null; then
  echo 'Compose environment secret leaked' >&2; exit 1
fi
echo 'PASS resolved Compose data volume / 使用 Compose 实际数据卷'

# OCI revision and binary health must pass before up/cleanup respectively.
: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_LABEL_REVISION=bbbbbbbbbbbb "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'mismatched build revision unexpectedly deployed' >&2; exit 1
fi
if grep -F ' up -d' "$LOG" >/dev/null; then
  echo 'compose up ran before build revision validation' >&2; exit 1
fi

: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_BINARY_HEALTH_FAIL=1 "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'failed binary health probe unexpectedly passed' >&2; exit 1
fi
if grep -F ' builder prune ' "$LOG" >/dev/null; then
  echo 'cleanup ran after failed binary health probe' >&2; exit 1
fi

# A healthy deployment removes only stopped managed/GSM objects and obsolete
# gsm-system tags; it preserves foreign tags and never operates on volumes.
: >"$LOG"
rm -rf "$TMP/state"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_CLEANUP_FIXTURES=1 "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F 'rm old-stopped' "$LOG" >/dev/null
grep -F 'image rm gsm-system:2.1.0-oldrevision' "$LOG" >/dev/null
grep -F 'image rm gsm-system:legacy' "$LOG" >/dev/null
grep -F 'image rm sha256:dangling' "$LOG" >/dev/null
grep -F 'image rm gsm-system:2.1.0' "$LOG" >/dev/null
if grep -F 'image rm lte:preserve' "$LOG" >/dev/null || grep -F 'volume rm' "$LOG" >/dev/null || grep -F 'system prune' "$LOG" >/dev/null; then
  echo 'cleanup escaped GSM runtime scope' >&2; exit 1
fi
echo 'PASS post-health GSM cleanup / 健康验收后清理 GSM 对象'

# Shared hosts can retain every cleanup object without disabling health gates.
# 共享服务器可跳过自动清理，但必须保留 HTTP 和二进制健康验收。
: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_CLEANUP_FIXTURES=1 "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build --skip-cleanup
grep -F 'exec fixture-current sh -c' "$LOG" >/dev/null
grep -F 'exec fixture-current /usr/local/bin/gsm-system --healthcheck' "$LOG" >/dev/null
if grep -E ' builder prune | image rm | rm old-stopped| image ls -aq' "$LOG" >/dev/null; then
  echo 'skip-cleanup performed cleanup operations' >&2; exit 1
fi
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_BINARY_HEALTH_FAIL=1 "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build --skip-cleanup; then
  echo 'skip-cleanup bypassed binary health gate' >&2; exit 1
fi
echo 'PASS skip-cleanup retains health gates / 跳过清理仍保留健康门禁'

# Exposure is an explicit env-file/Compose-resolved choice, never eval/source.
# 独立 API 端口仅由明确配置启用；环境文件不得执行。
: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_COMPOSE_EXPOSE_API=true "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F -- '-f deploy/docker/docker-compose.yml -f deploy/docker/docker-compose.api.yml up -d' "$LOG" >/dev/null
: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_DEPLOY_STATE=$TMP/state \
  FAKE_COMPOSE_EXPOSE_API=false "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
if grep -F 'docker-compose.api.yml' "$LOG" >/dev/null; then
  echo 'default deployment unexpectedly published separate API' >&2; exit 1
fi
: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_COMPOSE_EXPOSE_API=invalid \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'invalid API exposure flag accepted' >&2; exit 1
fi
if grep -F ' up -d' "$LOG" >/dev/null; then
  echo 'invalid exposure configuration started a container' >&2; exit 1
fi
echo 'PASS explicit API exposure and default Web-only policy / 显式 API 暴露及默认仅 Web 策略通过'
