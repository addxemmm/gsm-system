#!/bin/sh
# Isolated server-side image smoke test: no RF, USB, host network, or live volume.
# 服务器镜像隔离冒烟测试：不使用射频、USB、host 网络或线上数据卷。
set -eu

IMAGE=${1:-gsmsystem-uhd4:test}
TOKEN=gsm-image-smoke-token-fixture-v1
LABEL=com.addx.gsm-system.image-smoke
RUN_ID_RAW=$(od -An -N8 -tx1 /dev/urandom)
RUN_ID=$(printf '%s' "$RUN_ID_RAW" | tr -d ' \n')
CONTAINER=gsm-image-smoke-$RUN_ID
VOLUME=gsm-image-smoke-data-$RUN_ID
CONTAINER_CREATED=0
VOLUME_CREATED=0
ASTERISK_STARTED=0

fail() { echo "FAIL [$1] $2" >&2; exit 1; }
pass() { echo "PASS [$1] $2"; }

container_owned() {
  [ "$(docker inspect --format "{{ index .Config.Labels \"$LABEL\" }}" "$CONTAINER" 2>/dev/null)" = "$RUN_ID" ]
}

volume_owned() {
  [ "$(docker volume inspect --format "{{ index .Labels \"$LABEL\" }}" "$VOLUME" 2>/dev/null)" = "$RUN_ID" ]
}

cleanup() {
  rc=$?
  trap - EXIT HUP INT TERM
  set +e
  if [ "$rc" -ne 0 ] && [ "$CONTAINER_CREATED" -eq 1 ] && container_owned; then
    echo "--- isolated container logs / 隔离容器日志 ---" >&2
    docker logs "$CONTAINER" >&2
    docker exec "$CONTAINER" sh -c 'test ! -f /tmp/asterisk-smoke.log || cat /tmp/asterisk-smoke.log' >&2
  fi
  if [ "$ASTERISK_STARTED" -eq 1 ] && [ "$CONTAINER_CREATED" -eq 1 ] && container_owned; then
    docker exec "$CONTAINER" asterisk -rx 'core stop now' >/dev/null 2>&1
  fi
  if [ "$CONTAINER_CREATED" -eq 1 ] && container_owned; then
    if ! docker rm -f "$CONTAINER" >/dev/null; then
      echo "FAIL [CLEANUP] test container remains: $CONTAINER" >&2
      rc=1
    fi
  fi
  if [ "$VOLUME_CREATED" -eq 1 ] && volume_owned; then
    if ! docker volume rm "$VOLUME" >/dev/null; then
      echo "FAIL [CLEANUP] test volume remains: $VOLUME" >&2
      rc=1
    fi
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' HUP INT TERM

[ "$#" -le 1 ] || fail INPUT "usage: $0 [IMAGE_TAG]"
[ "${#RUN_ID}" -eq 16 ] || fail PREREQ "failed to generate a unique fixture id"
case "$IMAGE" in ''|-*) fail INPUT "invalid image tag" ;; esac
command -v docker >/dev/null 2>&1 || fail PREREQ "docker command not found"
docker image inspect "$IMAGE" >/dev/null 2>&1 || fail PREREQ "image not found: $IMAGE"
if docker container inspect "$CONTAINER" >/dev/null 2>&1; then
  fail COLLISION "test container already exists: $CONTAINER"
fi
if docker volume inspect "$VOLUME" >/dev/null 2>&1; then
  fail COLLISION "test volume already exists: $VOLUME"
fi

VOLUME_CREATED=1
docker volume create --label "$LABEL=$RUN_ID" "$VOLUME" >/dev/null || \
  fail CREATE "failed to create test volume"
volume_owned || fail OWNERSHIP "created volume label mismatch"

start_container() {
  CONTAINER_CREATED=1
  docker run -d \
    --name "$CONTAINER" \
    --label "$LABEL=$RUN_ID" \
    --network none \
    --init \
    -e "GSM_API_TOKEN=$TOKEN" \
    -e 'GSM_LISTEN=:8082' \
    -v "$VOLUME:/data" \
    "$IMAGE" >/dev/null || fail CREATE "failed to create isolated container"
  container_owned || fail OWNERSHIP "created container label mismatch"
}

wait_ready() {
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    if [ "$(docker inspect --format '{{.State.Running}}' "$CONTAINER")" = true ] && \
       [ "$(docker exec "$CONTAINER" curl -sS --max-time 2 -o /tmp/image-smoke-response.json \
          -w '%{http_code}' -H "Authorization: Bearer $TOKEN" \
          http://127.0.0.1:8082/api/v1/cell 2>/dev/null)" = 200 ]; then
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  fail STARTUP "isolated API did not become ready"
}

assert_marker_state() {
  openbts_marker=$(docker exec "$CONTAINER" sqlite3 -bail /etc/OpenBTS/OpenBTS.db \
    "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='ImageSmoke.Marker';")
  [ "$openbts_marker" = "$RUN_ID" ] || fail PERSISTENCE "OpenBTS marker mismatch"
  asterisk_marker=$(docker exec "$CONTAINER" sqlite3 -bail \
    /var/lib/asterisk/sqlite3dir/sqlite3.db \
    'SELECT marker FROM image_smoke_fixture WHERE id=1;')
  [ "$asterisk_marker" = "$RUN_ID" ] || fail PERSISTENCE "Asterisk marker mismatch"
  [ "$(docker exec "$CONTAINER" sqlite3 -bail /etc/OpenBTS/OpenBTS.db \
    'PRAGMA quick_check;')" = ok ] || fail SQLITE "OpenBTS quick_check failed"
  [ "$(docker exec "$CONTAINER" sqlite3 -bail \
    /var/lib/asterisk/sqlite3dir/sqlite3.db 'PRAGMA quick_check;')" = ok ] || \
    fail SQLITE "Asterisk quick_check failed"
}

start_container
wait_ready
pass STARTUP "isolated container ready (network=none, no RF devices)"

status=$(docker exec "$CONTAINER" curl -sS --max-time 5 \
  -o /tmp/image-smoke-response.json -w '%{http_code}' \
  http://127.0.0.1:8082/api/v1/cell)
[ "$status" = 401 ] || fail AUTH-401 "unauthenticated status was $status"
pass AUTH-401 "missing bearer token rejected"

status=$(docker exec "$CONTAINER" curl -sS --max-time 5 \
  -o /tmp/image-smoke-response.json -w '%{http_code}' \
  -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8082/api/v1/cell)
[ "$status" = 200 ] || fail CELL-STATUS "authenticated status was $status"
docker exec "$CONTAINER" grep -Eq '"running"[[:space:]]*:[[:space:]]*false' \
  /tmp/image-smoke-response.json || fail CELL-STATUS "cell was not reported stopped"
pass CELL-STATUS "authenticated cell status is running=false"

status=$(docker exec "$CONTAINER" curl -sS --max-time 5 \
  -o /tmp/image-smoke-response.json -w '%{http_code}' -X PATCH \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  --data-binary '{} {}' http://127.0.0.1:8082/api/v1/config)
[ "$status" = 400 ] || fail JSON-TRAILING "trailing JSON status was $status"
pass JSON-TRAILING "trailing JSON rejected without mutation"

status=$(docker exec "$CONTAINER" sh -ceu '
payload=/tmp/image-smoke-oversize.json
zeros=/tmp/image-smoke-zeros
printf "{\"iface\":\"eth0\"}" >"$payload"
dd if=/dev/zero of="$zeros" bs=65537 count=1 2>/dev/null
tr "\000" " " <"$zeros" >>"$payload"
rm -f "$zeros"
curl -sS --max-time 5 -o /tmp/image-smoke-response.json -w "%{http_code}" \
  -X POST -H "Authorization: Bearer $GSM_API_TOKEN" \
  -H "Content-Type: application/json" --data-binary @"$payload" \
  http://127.0.0.1:8082/api/v1/network
')
[ "$status" = 413 ] || fail JSON-LIMIT "oversize tail status was $status"
pass JSON-LIMIT "body limit includes a tail beyond 64 KiB"

[ "$(docker exec "$CONTAINER" sqlite3 -bail /etc/OpenBTS/OpenBTS.db \
  "SELECT count(*) FROM CONFIG WHERE KEYSTRING='ImageSmoke.Marker';")" = 0 ] || \
  fail FIXTURE-COLLISION "OpenBTS fixture key already exists"
[ "$(docker exec "$CONTAINER" sqlite3 -bail \
  /var/lib/asterisk/sqlite3dir/sqlite3.db \
  "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='image_smoke_fixture';")" = 0 ] || \
  fail FIXTURE-COLLISION "Asterisk fixture table already exists"
docker exec "$CONTAINER" sqlite3 -bail /etc/OpenBTS/OpenBTS.db \
  "INSERT INTO CONFIG(KEYSTRING,VALUESTRING,STATIC,OPTIONAL,COMMENTS) VALUES('ImageSmoke.Marker','$RUN_ID',1,1,'isolated image smoke fixture');"
docker exec "$CONTAINER" sqlite3 -bail /var/lib/asterisk/sqlite3dir/sqlite3.db \
  "CREATE TABLE image_smoke_fixture(id INTEGER PRIMARY KEY, marker TEXT NOT NULL); INSERT INTO image_smoke_fixture VALUES(1,'$RUN_ID');"
assert_marker_state
pass SQLITE-WRITE "isolated OpenBTS and Asterisk markers written"

docker restart --time 30 "$CONTAINER" >/dev/null
wait_ready
assert_marker_state
pass RESTART-PERSISTENCE "markers survived docker restart"

container_owned || fail OWNERSHIP "refusing to remove an unowned container"
docker rm -f "$CONTAINER" >/dev/null
CONTAINER_CREATED=0
if docker container inspect "$CONTAINER" >/dev/null 2>&1; then
  fail REMOVE "test container still exists after removal"
fi
start_container
wait_ready
assert_marker_state
pass RECREATE-PERSISTENCE "markers survived container recreation on the same test volume"

docker exec "$CONTAINER" grep -Eq 'res_config_odbc\.so' /etc/asterisk/modules.conf || \
  fail ODBC-CONFIG "res_config_odbc entry missing"
if docker exec "$CONTAINER" grep -Eq \
  '^[[:space:]]*noload[[:space:]]*=>[[:space:]]*res_config_odbc\.so' \
  /etc/asterisk/modules.conf; then
  fail ODBC-CONFIG "res_config_odbc has an active noload"
fi
pass ODBC-CONFIG "res_config_odbc is not disabled"

docker exec -d "$CONTAINER" sh -c \
  'exec asterisk -f -g >/tmp/asterisk-smoke.log 2>&1'
ASTERISK_STARTED=1
attempt=0
odbc_status=
while [ "$attempt" -lt 30 ]; do
  if odbc_status=$(docker exec "$CONTAINER" asterisk -rx \
      'module show like res_config_odbc' 2>/dev/null) && \
     printf '%s\n' "$odbc_status" | grep -Eq 'res_config_odbc\.so.*Running'; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
printf '%s\n' "$odbc_status" | grep -Eq 'res_config_odbc\.so.*Running' || \
  fail ODBC-RUNTIME "res_config_odbc did not reach Running"
docker exec "$CONTAINER" asterisk -rx 'core stop now' >/dev/null
ASTERISK_STARTED=0
pass ODBC-RUNTIME "res_config_odbc loaded; isolated Asterisk stopped"

pass IMAGE-SMOKE "all isolated checks passed for $IMAGE"
