#!/bin/sh
# Isolated server-side image smoke test: no RF, USB, host network, or live volume.
# 服务器镜像隔离冒烟测试：不使用射频、USB、host 网络或线上数据卷。
set -eu

IMAGE=${1:-gsm-system:2.1.0}
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
    docker exec "$CONTAINER" sh -c 'test ! -f /tmp/image-smoke-calls.json || { echo "--- active calls ---"; cat /tmp/image-smoke-calls.json; }' >&2
    docker exec "$CONTAINER" sh -c 'test ! -f /tmp/image-smoke-history.json || { echo "--- call history ---"; cat /tmp/image-smoke-history.json; }' >&2
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
IMAGE_VERSION=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.version" }}' "$IMAGE")
IMAGE_REVISION=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "$IMAGE")
[ "$IMAGE_VERSION" = 2.1.0 ] || fail IDENTITY "unexpected OCI version: $IMAGE_VERSION"
case "$IMAGE_REVISION" in ''|*[!0-9a-f]*) fail IDENTITY "invalid OCI revision: $IMAGE_REVISION" ;; esac
[ "${#IMAGE_REVISION}" -eq 12 ] || fail IDENTITY "OCI revision is not 12 characters: $IMAGE_REVISION"
[ "$(docker run --rm --entrypoint /usr/local/bin/gsm-system "$IMAGE" --version)" = \
  "gsm-system $IMAGE_VERSION ($IMAGE_REVISION)" ] || fail IDENTITY "binary and OCI identity differ"
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
  [ "$(docker exec "$CONTAINER" cat /var/log/asterisk/cdr-csv/image-smoke.cdr)" = "$RUN_ID" ] || \
    fail PERSISTENCE "Asterisk CDR marker mismatch"
}

start_container
wait_ready
docker exec "$CONTAINER" /usr/local/bin/gsm-system --healthcheck || fail HEALTH "stopped cell health probe failed"
pass STARTUP "isolated container ready (network=none, no RF devices)"
docker exec "$CONTAINER" test -S /dev/log || fail LOGGING "Go syslog socket /dev/log is missing"
docker exec "$CONTAINER" test ! -e /OpenBTS/run.so || fail GO-ONLY "legacy run.so is present"
docker exec "$CONTAINER" test ! -e /OpenBTS/run.py || fail GO-ONLY "legacy run.py is present"
for pinned in \
  'cppzmq:76bf169fd67b8e99c1b0e6490029d9cd5ef97666' \
  'liba53:27354560dc7b554e03d40a520d41290e731193b6' \
  'libcoredumper:7527fb3804927c7fdc72ff5139a2cdea3db4d59a' \
  'openbts:7766ef94f2d885c197430e74a89f02740f0c04e6' \
  'smqueue:e168a262db311231c51cf7295f9bd1f440567485' \
  'subscriberRegistry:c65b5d59f744a8df5f3395e217f2598f53e9fe65' \
  'uhd4:d21735d543d5a3c265507965c7bb6c9e9df95fcd'
do
  component=${pinned%%:*}
  revision=${pinned#*:}
  docker exec "$CONTAINER" awk -F '\t' -v component="$component" -v revision="$revision" \
    '$1 == component && $2 == "repository" && $4 == revision { found++ } END { exit found == 1 ? 0 : 1 }' \
    /usr/share/doc/gsm-system/VENDOR-REVISIONS.tsv || \
    fail PROVENANCE "missing pinned vendor row: $pinned"
done
pass IDENTITY "OCI labels, Go binary and vendor provenance agree"

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
  -X PUT -H "Authorization: Bearer $GSM_API_TOKEN" \
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
docker exec "$CONTAINER" sh -c 'printf "%s\n" "$1" >/var/log/asterisk/cdr-csv/image-smoke.cdr' sh "$RUN_ID"
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
docker exec "$CONTAINER" grep -Eq '^[[:space:]]*usegmtime[[:space:]]*=[[:space:]]*yes' /etc/asterisk/cdr.conf || \
  fail CDR-CONFIG "CDR UTC timestamps are not enabled"
docker exec "$CONTAINER" grep -Eq '^[[:space:]]*batch[[:space:]]*=[[:space:]]*no' /etc/asterisk/cdr.conf || \
  fail CDR-CONFIG "CDR batch mode must be disabled for durable writes"
pass CDR-CONFIG "Asterisk CDR history uses UTC and durable writes"

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
for module in cdr_csv cdr_custom; do
  module_status=$(docker exec "$CONTAINER" asterisk -rx "module show like $module" 2>/dev/null) || \
    fail CDR-RUNTIME "failed to query $module"
  printf '%s\n' "$module_status" | grep -Eq "$module\.so.*Running" || \
    fail CDR-RUNTIME "$module did not reach Running"
done
cdr_status=$(docker exec "$CONTAINER" asterisk -rx 'cdr show status' 2>/dev/null) || \
  fail CDR-RUNTIME "failed to query CDR status"
printf '%s\n' "$cdr_status" | grep -Eqi '^[[:space:]]*Logging:[[:space:]]*Enabled[[:space:]]*$' || \
  fail CDR-RUNTIME "CDR logging is not enabled"

# Reload and resolve representative contexts/extensions. This makes parse or
# include errors observable without starting OpenBTS or touching RF.
# 重载并解析代表性上下文/号码；无需启动 OpenBTS 或射频即可发现 dialplan 错误。
dialplan_reload=$(docker exec "$CONTAINER" asterisk -rx 'dialplan reload' 2>&1) || {
  printf '%s\n' "$dialplan_reload" >&2
  fail DIALPLAN "dialplan reload failed"
}
for target in from-openBTS phones to-pstn default 112@emergency 911@emergency; do
  dialplan_output=$(docker exec "$CONTAINER" asterisk -rx "dialplan show $target" 2>&1) || {
    printf '%s\n' "$dialplan_output" >&2
    fail DIALPLAN "dialplan target did not load: $target"
  }
  if printf '%s\n' "$dialplan_output" | grep -Eqi 'there is no existence of|no such context|not found'; then
    printf '%s\n' "$dialplan_output" >&2
    fail DIALPLAN "dialplan target is absent: $target"
  fi
done
if docker exec "$CONTAINER" grep -Eqi \
  'parse error|unable to include context|unable to register extension|failed to load.*extensions\.conf' \
  /tmp/asterisk-smoke.log; then
  docker exec "$CONTAINER" grep -Ein \
    'parse error|unable to include context|unable to register extension|failed to load.*extensions\.conf' \
    /tmp/asterisk-smoke.log >&2
  fail DIALPLAN "Asterisk reported a dialplan parse/load error"
fi
pass ASTERISK-RUNTIME "ODBC/CDR modules and dialplan loaded without parse errors"

# Create an isolated Local-channel call. No SIP/OpenBTS/RF endpoint is named or
# reachable (the container remains network=none and has no radio device).
# 仅创建 Asterisk Local 通道：不引用 SIP/OpenBTS/RF，容器仍为 network=none 且无射频设备。
docker exec "$CONTAINER" sh -ceu '
id=$1
cat >>/etc/asterisk/extensions.conf <<EOF

[image-smoke-local]
exten => probe,1,Set(CDR(userfield)=image-smoke-$id)
 same => n,Set(CALLERID(num)=$id)
 same => n,Answer()
 same => n,Dial(Local/sink@image-smoke-local/n,8)
 same => n,Hangup()
exten => sink,1,Set(CDR(userfield)=image-smoke-$id)
 same => n,Answer()
 same => n,Wait(6)
 same => n,Hangup()
EOF
' sh "$RUN_ID"
channeltypes=$(docker exec "$CONTAINER" asterisk -rx 'core show channeltypes' 2>/dev/null) || \
  fail LOCAL-CALL "failed to query channel types"
printf '%s\n' "$channeltypes" | grep -Eq '^[[:space:]]*Local[[:space:]]' || \
  fail LOCAL-CALL "core Local channel technology is unavailable"
module_status=$(docker exec "$CONTAINER" asterisk -rx 'module show like app_dial' 2>/dev/null) || \
  fail LOCAL-CALL "failed to query app_dial"
printf '%s\n' "$module_status" | grep -Eq 'app_dial\.so.*Running' || \
  fail LOCAL-CALL "app_dial did not reach Running"
dialplan_reload=$(docker exec "$CONTAINER" asterisk -rx 'dialplan reload' 2>&1) || {
  printf '%s\n' "$dialplan_reload" >&2
  fail LOCAL-CALL "fixture dialplan reload failed"
}
fixture_dialplan=$(docker exec "$CONTAINER" asterisk -rx 'dialplan show image-smoke-local' 2>&1) || {
  printf '%s\n' "$fixture_dialplan" >&2
  fail LOCAL-CALL "fixture dialplan did not load"
}
printf '%s\n' "$fixture_dialplan" | grep -Eq "'probe'.*Set|probe.*Set" || \
  fail LOCAL-CALL "fixture probe extension is absent"
printf '%s\n' "$fixture_dialplan" | grep -Eq "'sink'.*Set|sink.*Set" || \
  fail LOCAL-CALL "fixture sink extension is absent"

originate_output=$(docker exec "$CONTAINER" asterisk -rx \
  'channel originate Local/probe@image-smoke-local/n application Wait 8' 2>&1) || {
  printf '%s\n' "$originate_output" >&2
  fail LOCAL-CALL "Local-channel originate failed"
}

# Observe the real Asterisk concise output through the REST parser while the
# Dial(Local/...) bridge is alive. HTTP 200 plus these fields guards against a
# silently empty result or parser error, without requiring jq in the image.
active_ok=0
attempt=0
while [ "$attempt" -lt 20 ]; do
  active_status=$(docker exec "$CONTAINER" curl -sS --max-time 5 \
    -o /tmp/image-smoke-calls.json -w '%{http_code}' \
    -H "Authorization: Bearer $TOKEN" \
    http://127.0.0.1:8082/api/v1/calls 2>/dev/null || true)
  if [ "$active_status" = 200 ] && \
     docker exec "$CONTAINER" grep -Eq '"count"[[:space:]]*:[[:space:]]*[1-9][0-9]*' /tmp/image-smoke-calls.json && \
     docker exec "$CONTAINER" grep -Eq '"channel"[[:space:]]*:[[:space:]]*"Local/[^" ]+"' /tmp/image-smoke-calls.json && \
     docker exec "$CONTAINER" grep -Eq '"bridge_id"[[:space:]]*:[[:space:]]*"[^" ]+"' /tmp/image-smoke-calls.json && \
     docker exec "$CONTAINER" grep -Eq '"unique_id"[[:space:]]*:[[:space:]]*"[^" ]+"' /tmp/image-smoke-calls.json; then
    active_ok=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
[ "$active_ok" -eq 1 ] || fail LOCAL-CALL "REST active-call snapshot lacked Local channel/bridge_id/unique_id"
pass CALLS-REST "live Local bridge parsed with non-empty channel, bridge_id and unique_id"

# Wait for the answered call to finish and cdr_csv to flush (batch=no), then
# require REST to parse the real 18-column Master.csv record carrying our marker.
history_ok=0
attempt=0
while [ "$attempt" -lt 20 ]; do
  history_status=$(docker exec "$CONTAINER" curl -sS --max-time 5 \
    -o /tmp/image-smoke-history.json -w '%{http_code}' \
    -H "Authorization: Bearer $TOKEN" \
    'http://127.0.0.1:8082/api/v1/calls/history?limit=20' 2>/dev/null || true)
  if [ "$history_status" = 200 ] && \
     docker exec "$CONTAINER" grep -Eq '"count"[[:space:]]*:[[:space:]]*[1-9][0-9]*' /tmp/image-smoke-history.json && \
     docker exec "$CONTAINER" grep -Fq '"source":"Master.csv"' /tmp/image-smoke-history.json && \
     docker exec "$CONTAINER" grep -Eq '"channel"[[:space:]]*:[[:space:]]*"Local/[^" ]+"' /tmp/image-smoke-history.json && \
     docker exec "$CONTAINER" grep -Fq '"destination_context":"image-smoke-local"' /tmp/image-smoke-history.json && \
     docker exec "$CONTAINER" grep -Fq '"disposition":"ANSWERED"' /tmp/image-smoke-history.json && \
     docker exec "$CONTAINER" grep -Eq '"unique_id"[[:space:]]*:[[:space:]]*"[^" ]+"' /tmp/image-smoke-history.json && \
     docker exec "$CONTAINER" grep -Fq "\"user_field\":\"image-smoke-$RUN_ID\"" /tmp/image-smoke-history.json; then
    history_ok=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
[ "$history_ok" -eq 1 ] || fail CALL-HISTORY "REST did not parse the marked 18-column CDR record"
pass CALL-HISTORY "real answered Local-call CDR parsed from Master.csv"

docker exec "$CONTAINER" asterisk -rx 'core stop now' >/dev/null
ASTERISK_STARTED=0
pass ODBC-RUNTIME "res_config_odbc loaded; isolated Asterisk stopped"

pass IMAGE-SMOKE "all isolated checks passed for $IMAGE"
