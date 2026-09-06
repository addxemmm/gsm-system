#!/bin/sh
# Offline contract tests: fake docker/curl/sleep, no network or daemon.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
TMP=${TMPDIR:-/tmp}/gsm-deploy-test-$$
BIN=$TMP/bin
LOG=$TMP/native.log
mkdir -p "$BIN"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT HUP INT TERM

cat >"$BIN/docker" <<'EOF'
#!/bin/sh
printf 'docker %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
case " $* " in
  *" build "*) [ "${FAKE_BUILD_FAIL:-0}" != 1 ] ;;
  *" config --images "*) echo 'gsm-fixture:test' ;;
  *" image inspect "*) echo 'sha256:fixture' ;;
  *" ps --all -q "*) echo 'fixture-container' ;;
  *"{{.State.Running}}"*) echo "${FAKE_RUNNING:-true}" ;;
  *"{{.Image}}"*) echo "${FAKE_IMAGE:-sha256:fixture}" ;;
  *) exit 0 ;;
esac
EOF
cat >"$BIN/curl" <<'EOF'
#!/bin/sh
printf 'curl %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
printf '{"status":"ok"}'
EOF
cat >"$BIN/sleep" <<'EOF'
#!/bin/sh
printf 'sleep %s\n' "$*" >>"$FAKE_DEPLOY_LOG"
EOF
chmod +x "$BIN/docker" "$BIN/curl" "$BIN/sleep"

PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG HEALTH_RETRIES=2 \
  "$ROOT/scripts/deploy_to_ubuntu.sh"
grep -F "docker compose -p gsm-system-live -f deploy/docker/docker-compose.uhd4.yml build" "$LOG" >/dev/null
grep -F "docker compose -p gsm-system-live -f deploy/docker/docker-compose.uhd4.yml up -d" "$LOG" >/dev/null
grep -F "curl -fsS --max-time 30 http://127.0.0.1:8082/api/v1/cell" "$LOG" >/dev/null

: >"$LOG"
if PATH=$BIN:$PATH FAKE_DEPLOY_LOG=$LOG FAKE_BUILD_FAIL=1 \
  "$ROOT/scripts/deploy_to_ubuntu.sh" --compose-file deploy/docker/docker-compose.yml --skip-health; then
  echo "expected failed docker build to fail the deploy" >&2
  exit 1
fi
grep -F "docker compose -p gsm-system-live -f deploy/docker/docker-compose.yml build" "$LOG" >/dev/null
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

echo "PASS test_deploy_to_ubuntu.sh / Ubuntu 部署脚本测试通过"
