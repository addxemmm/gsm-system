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
  *" compose "*" build "*) [ "${FAKE_BUILD_FAIL:-0}" != 1 ] ;;
  *" compose "*" config --images "*) echo "$GSM_IMAGE" ;;
  *" compose "*" ps --all -q "*) echo 'fixture-container' ;;
  *" volume inspect "*) exit 0 ;;
  *" image inspect "*"org.opencontainers.image.version"*) echo "${FAKE_LABEL_VERSION:-$GSM_VERSION}" ;;
  *" image inspect "*"org.opencontainers.image.revision"*) echo "${FAKE_LABEL_REVISION:-$GSM_REVISION}" ;;
  *" image inspect "*) echo 'sha256:fixture' ;;
  *" run --rm --entrypoint /usr/local/bin/gsm-system "*) echo "gsm-system $GSM_VERSION ($GSM_REVISION)" ;;
  *"{{.State.Running}}"*) echo "${FAKE_RUNNING:-true}" ;;
  *"{{.Image}}"*) echo "${FAKE_IMAGE:-sha256:fixture}" ;;
  *) exit 0 ;;
esac
EOF
cat >"$BIN/curl" <<'EOF'
#!/bin/sh
printf 'curl %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
[ "${FAKE_CURL_FAIL:-0}" != 1 ] || exit 22
printf '{"status":"ok"}'
EOF
cat >"$BIN/sleep" <<'EOF'
#!/bin/sh
printf 'sleep %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
EOF
chmod +x "$BIN/docker" "$BIN/curl" "$BIN/sleep"

PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG HEALTH_RETRIES=2 \
  "$ROOT/scripts/deploy_to_ubuntu.sh"
grep -F "image=$IMAGE version=2.1.0 revision=$REVISION compose -p gsm-system-live -f deploy/docker/docker-compose.yml build" "$LOG" >/dev/null
grep -F "compose -p gsm-system-live -f deploy/docker/docker-compose.yml up -d" "$LOG" >/dev/null
grep -F "curl -fsS --max-time 30 http://127.0.0.1:8082/api/v1/cell" "$LOG" >/dev/null
grep -F "image tag $IMAGE gsm-system:2.1.0" "$LOG" >/dev/null
curl_line=$(grep -n '^curl ' "$LOG" | tail -1 | cut -d: -f1)
tag_line=$(grep -n "image tag $IMAGE gsm-system:2.1.0" "$LOG" | cut -d: -f1)
[ "$tag_line" -gt "$curl_line" ] || { echo 'release alias was tagged before HTTP validation' >&2; exit 1; }

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
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_RUNNING=false \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'exited container incorrectly passed readiness' >&2; exit 1
fi
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_IMAGE=sha256:stale \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --skip-build; then
  echo 'stale image incorrectly passed readiness' >&2; exit 1
fi

: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_CURL_FAIL=1 HEALTH_RETRIES=1 \
  "$ROOT/scripts/deploy_to_ubuntu.sh"; then
  echo 'failed HTTP readiness incorrectly passed' >&2; exit 1
fi
if grep -F ' image tag ' "$LOG" >/dev/null; then
  echo 'release alias was tagged after failed HTTP readiness' >&2; exit 1
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
