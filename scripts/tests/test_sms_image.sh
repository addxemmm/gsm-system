#!/bin/sh
# SDR build server only: isolated SMS history tests, no RF/USB/published ports.
# 仅在构建服务器执行：隔离短信日志查询与去重，不连接手机或发送短信。
set -eu
[ "$#" -eq 1 ] || { echo 'Usage: sh scripts/tests/test_sms_image.sh IMAGE' >&2; exit 2; }
image=$1
docker image inspect "$image" >/dev/null
tmp=$(mktemp -d "${TMPDIR:-/tmp}/gsm-sms-image.XXXXXX")
name=gsm-sms-image-${tmp##*.}
volume=$name-data
cleanup() {
  rc=$?
  trap - EXIT HUP INT TERM
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  rm -f "$tmp/app.yaml" "$tmp/smqueue.log"
  rmdir "$tmp"
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
printf 'uhd_find_bin: /bin/false\nsyslog_socket: ""\n' >"$tmp/app.yaml"
docker volume create "$volume" >/dev/null
docker run -d --name "$name" --network none \
  -e GSM_CONFIG=/fixture/app.yaml -e GSM_API_TOKEN= \
  --mount "type=bind,source=$tmp/app.yaml,target=/fixture/app.yaml,readonly" \
  --mount "type=volume,source=$volume,target=/data" \
  --entrypoint /usr/local/bin/gsm-system "$image" >/dev/null
tries=0
until docker exec "$name" curl -fsS --max-time 3 http://127.0.0.1:8082/api/v1/health >/dev/null 2>&1; do
  tries=$((tries+1))
  [ "$tries" -lt 30 ] || { echo 'API readiness failed' >&2; exit 1; }
  sleep 1
done
hex() { printf '%s' "$1" | od -An -tx1 | tr -d ' \n'; }
sender=$(hex IMSI001010000000001)
receiver=$(hex 10002)
body=$(hex hello)
for tag in 41 42 41; do
  printf '<189>Sep 8 10:00:00 smqueue: NOTICE 1:2 2026-09-08T10:00:00.0 smsc.cpp:320:submitSMS: GSM_SMS_V1 qtag_hex=%s from_hex=%s to_hex=%s text_hex=%s\n' \
    "$tag" "$sender" "$receiver" "$body"
done >"$tmp/smqueue.log"
# A marker inside an unkeyed body must not create a third forged event.
printf '<189>Sep 8 10:00:01 smqueue: NOTICE 1:2 2026-09-08T10:00:01.0 smqueue.h:505:get_text: Decoded text: GSM_SMS_V1 qtag_hex=43 from_hex=313031 to_hex=313032 text_hex=78\n' >>"$tmp/smqueue.log"
docker cp "$tmp/smqueue.log" "$name:/data/log/smqueue.log" >/dev/null
response=$(docker exec "$name" curl -fsS --max-time 10 http://127.0.0.1:8082/api/v1/sms)
printf '%s' "$response" | grep -Fq '"total":2'
[ "$(printf '%s' "$response" | grep -o '"text":"hello"' | wc -l | tr -d '[:space:]')" -eq 2 ]
printf '%s' "$response" | grep -Fq '"receiver_number":"10002"'
printf '%s' "$response" | grep -Fq '"sender_imsi":"001010000000001"'
response=$(docker exec "$name" curl -fsS --max-time 10 'http://127.0.0.1:8082/api/v1/sms?limit=1&offset=1')
printf '%s' "$response" | grep -Fq '"count":1'
printf '%s' "$response" | grep -Fq '"total":2'
echo 'PASS image SMS history, identity deduplication, body injection rejection and pagination; no RF / 镜像短信历史、身份去重、防正文伪日志与分页通过，无射频'
