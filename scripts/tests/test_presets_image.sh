#!/bin/sh
# Run only on the SDR build server, never on the development machine.
# 仅在 SDR 构建服务器执行；独立测试卷、无 USB、无特权、不启动射频。
set -eu
[ "$#" -eq 1 ] || { echo 'Usage: sh scripts/tests/test_presets_image.sh IMAGE' >&2; exit 2; }
image=$1
docker image inspect "$image" >/dev/null
tmp=$(mktemp -d "${TMPDIR:-/tmp}/gsm-presets-image.XXXXXX")
name=gsm-presets-image-${tmp##*.}
volume=$name-data
cleanup() {
  rc=$?
  trap - EXIT HUP INT TERM
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  rm -f "$tmp/app.yaml"
  rmdir "$tmp"
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
printf 'uhd_find_bin: /bin/false\nsyslog_socket: ""\n' >"$tmp/app.yaml"
docker volume create "$volume" >/dev/null
docker run -d --name "$name" --network bridge \
  -e GSM_CONFIG=/fixture/app.yaml -e GSM_API_TOKEN= \
  --mount "type=bind,source=$tmp/app.yaml,target=/fixture/app.yaml,readonly" \
  --mount "type=volume,source=$volume,target=/data" \
  --entrypoint /usr/local/bin/gsm-system "$image" >/dev/null

ready() {
  count=0
  until docker exec "$name" curl -fsS http://127.0.0.1:8082/api/v1/cell >/dev/null 2>&1; do
    count=$((count + 1))
    [ "$count" -lt 30 ] || { echo 'API readiness failed' >&2; exit 1; }
    sleep 1
  done
}
request() {
  method=$1 path=$2 expected=$3
  shift 3
  if [ "$#" -gt 0 ]; then
    status=$(docker exec "$name" curl -sS --max-time 30 -o /tmp/response.json -w '%{http_code}' \
      -X "$method" -H 'Content-Type: application/json' --data "$1" "http://127.0.0.1:8082$path")
  else
    status=$(docker exec "$name" curl -sS --max-time 30 -o /tmp/response.json -w '%{http_code}' \
      -X "$method" "http://127.0.0.1:8082$path")
  fi
  [ "$status" = "$expected" ] || { echo "$method $path: $status, expected $expected" >&2; exit 1; }
  response=$(docker exec "$name" cat /tmp/response.json)
}
ready
params='{"arfcns":"1","c0":"55","band":"900","mcc":"001","mnc":"01","lac":"1","ci":"1","short_name":"LAB","network":"eth0"}'
request GET /api/v1/presets 200
[ "$(printf '%s' "$response" | grep -o '"id":' | wc -l | tr -d '[:space:]')" = 5 ]
for default_id in 0 1 2 3 4; do
  request GET "/api/v1/presets/$default_id" 200
  for field in '"arfcns":"1"' '"lac":"1"' '"ci":"1"' '"network":"eth0"'; do
    printf '%s' "$response" | grep -Fq "$field"
  done
done
request GET /api/v1/presets/0 200
printf '%s' "$response" | grep -Fq '"name":"addx"'
printf '%s' "$response" | grep -Fq '"short_name":"addx"'
# No description is required. / description 可省略。
request POST /api/v1/presets 201 "{\"id\":\"lab-900\",\"name\":\"Lab\",\"params\":$params}"
request PUT /api/v1/presets/lab-900 200 "{\"name\":\"Updated\",\"params\":$params}"
[ "$(docker exec "$name" stat -c '%a' /data/presets.json)" = 600 ]
docker restart "$name" >/dev/null
ready
request GET /api/v1/presets/lab-900 200
printf '%s' "$response" | grep -q '"name":"Updated"'
# Resolve both start modes with a deliberately disabled hardware detector.
# 验证两种启动路径，检测器固定失败，绝不访问真实射频硬件。
request POST /api/v1/cell 503 '{"preset_id":"lab-900"}'
request POST /api/v1/cell 503 '{"preset_id":"0"}'
request POST /api/v1/cell 503 "$params"
request POST /api/v1/cell 422 '{"preset_id":"lab-900","network":null}'
request GET /api/v1/cell 200
printf '%s' "$response" | grep -q '"state":"stopped"'
request GET /config 404
request DELETE /api/v1/presets/lab-900 200
request GET /api/v1/presets/lab-900 404
# Deliberate fixture-only deletion survives restart; defaults are seeded once.
# 仅删除独立测试卷中的默认项，验证重启不覆盖用户删除决定。
request DELETE /api/v1/presets/4 200
docker restart "$name" >/dev/null
ready
request GET /api/v1/presets/4 404
request GET /api/v1/presets/0 200
echo 'PASS five built-in defaults, persistence, CRUD and both start modes; no RF / 五套内置预设及持久化、双启动路径通过，未启动射频'
