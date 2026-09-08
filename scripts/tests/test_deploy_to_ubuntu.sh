#!/bin/sh
# Offline contract tests: fake docker/curl/sleep, no network or daemon.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
TMP=${TMPDIR:-/tmp}/gsm-deploy-test-$$
BIN=$TMP/bin
LOG=$TMP/native.log
REVISION=$(git -C "$ROOT" rev-parse --short=12 HEAD)
IMAGE=gsm-system:2.1.0-$REVISION
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
    ;;
  *" compose "*" build "*) [ "${FAKE_BUILD_FAIL:-0}" != 1 ] ;;
  *" compose "*" config --images "*) echo "$GSM_IMAGE" ;;
  *" compose "*" config --services "*) echo 'gsm-system' ;;
  *" compose "*" ps --all -q gsm-system "*) echo 'fixture-current' ;;
  *" compose "*" ps --all -q "*) printf '%s\n' 'fixture-current' 'rollback-orphan' ;;
  *" volume inspect "*) exit 0 ;;
  *" image inspect "*"org.opencontainers.image.version"*) echo "${FAKE_LABEL_VERSION:-$GSM_VERSION}" ;;
  *" image inspect "*"org.opencontainers.image.revision"*) echo "${FAKE_LABEL_REVISION:-$GSM_REVISION}" ;;
  *" image inspect "*) echo 'sha256:fixture' ;;
  *" run --rm --entrypoint /usr/local/bin/gsm-system "*) echo "gsm-system $GSM_VERSION ($GSM_REVISION)" ;;
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
  *"{{.Image}}"*" rollback-orphan "*) echo "${FAKE_ORPHAN_IMAGE:-sha256:rollback}" ;;
  *"{{.Image}}"*" fixture-current "*) echo "${FAKE_CURRENT_IMAGE:-sha256:fixture}" ;;
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
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG HEALTH_RETRIES=2 \
  FAKE_ORPHAN_RUNNING=false FAKE_ORPHAN_IMAGE=sha256:rollback \
  "$ROOT/scripts/deploy_to_ubuntu.sh"
grep -F "image=$IMAGE version=2.1.0 revision=$REVISION compose -p gsm-system-live -f deploy/docker/docker-compose.yml build" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml config --services" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml ps --all -q gsm-system" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml up -d" "$LOG" >/dev/null
grep -F 'exec fixture-current sh -c' "$LOG" >/dev/null
grep -F 'printf "Authorization: Bearer %s\n" "$GSM_API_TOKEN"' "$LOG" >/dev/null
grep -F 'curl -fsS --max-time 30 -H @-' "$LOG" >/dev/null
grep -F 'http://127.0.0.1:8082/api/v1/cell' "$LOG" >/dev/null
grep -F "image tag $IMAGE gsm-system:2.1.0" "$LOG" >/dev/null
curl_line=$(grep -n 'exec fixture-current sh -c' "$LOG" | tail -1 | cut -d: -f1)
tag_line=$(grep -n "image tag $IMAGE gsm-system:2.1.0" "$LOG" | cut -d: -f1)
[ "$tag_line" -gt "$curl_line" ] || { echo 'release alias was tagged before HTTP validation' >&2; exit 1; }
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
if grep -F ' image tag ' "$LOG" >/dev/null; then
  echo 'release alias was tagged after failed HTTP readiness' >&2; exit 1
fi

# A token that exists only in the Compose env file must be consumed inside the
# service container, without sourcing the file or exposing its value in logs.
# 仅 Compose 环境文件有令牌时，探针须在容器内取值且不得泄露。
: >"$LOG"
TOKEN_ENV=$TMP/token.env
TOKEN_FIXTURE=fixture-token-do-not-log
printf 'GSM_API_TOKEN=%s\n' "$TOKEN_FIXTURE" >"$TOKEN_ENV"
env -u GSM_API_TOKEN "PATH=$BIN:$PATH" "FAKE_DEPLOY_LOG=$LOG" \
  "GSM_ENV_FILE=$TOKEN_ENV" FAKE_REQUIRE_CONTAINER_TOKEN=1 \
  "FAKE_CONTAINER_TOKEN=$TOKEN_FIXTURE" \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F "compose --env-file $TOKEN_ENV -p gsm-system-live" "$LOG" >/dev/null
grep -F 'printf "Authorization: Bearer %s\n" "$GSM_API_TOKEN"' "$LOG" >/dev/null
if grep -F "$TOKEN_FIXTURE" "$LOG" >/dev/null; then
  echo 'container token leaked into deployment log' >&2; exit 1
fi

# Explicit immutable images are rollback inputs: infer their revision and never
# repoint the mutable release alias. / 回滚镜像自动推导 revision，且不更新别名。
: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG GSM_IMAGE=gsm-system:2.1.0-aaaaaaaaaaaa \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build --skip-health
grep -F 'revision=aaaaaaaaaaaa' "$LOG" >/dev/null
if grep -F ' image tag ' "$LOG" >/dev/null; then
  echo 'rollback unexpectedly changed the release alias' >&2; exit 1
fi

echo "PASS test_deploy_to_ubuntu.sh / Ubuntu 部署脚本测试通过"

: >"$LOG"
PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_COMPOSE_VOLUME=fixture-env-volume \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build
grep -F 'volume inspect fixture-env-volume' "$LOG" >/dev/null
if grep -F 'fixture-secret-not-printed' "$LOG" >/dev/null; then
  echo 'Compose environment secret leaked' >&2; exit 1
fi
echo 'PASS resolved Compose data volume / 使用 Compose 实际数据卷'
