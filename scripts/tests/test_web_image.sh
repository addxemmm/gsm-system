#!/bin/sh
# Isolated Web/API final-image acceptance: no USB, RF, host ports or live data.
# 最终镜像 Web/API 隔离验收：无 USB、射频、宿主端口或真实业务数据。
set -eu
image=${1:-gsm-system:2.1}
run_id=$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')
name=gsm-web-smoke-$run_id
cleanup() {
  rc=$?
  trap - EXIT HUP INT TERM
  if [ "$(docker inspect -f '{{ index .Config.Labels "com.gsm-system.web-fixture" }}' "$name" 2>/dev/null)" = "$run_id" ]; then
    docker rm -f "$name" >/dev/null 2>&1 || rc=1
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

# Resolve real Compose merges without printing env/token or starting services.
# 检查真实 Compose 合并，不打印环境密钥、不启动服务。
base=$(GSM_BIND_ADDRESS=127.0.0.1 GSM_WEB_PORT=18080 GSM_API_PORT=18082 \
  docker compose --env-file /dev/null -f deploy/docker/docker-compose.yml config --format json | tr -d '[:space:]')
printf '%s' "$base" | grep -Fq '"published":"18080"'
if printf '%s' "$base" | grep -Fq '"published":"18082"'; then
  echo 'FAIL standalone API published by default' >&2; exit 1
fi
printf '%s' "$base" | grep -Fq '"GSM_LISTEN":"127.0.0.1:8082"'
both=$(GSM_BIND_ADDRESS=127.0.0.1 GSM_WEB_PORT=18080 GSM_API_PORT=18082 \
  docker compose --env-file /dev/null -f deploy/docker/docker-compose.yml \
  -f deploy/docker/docker-compose.api.yml config --format json | tr -d '[:space:]')
printf '%s' "$both" | grep -Fq '"published":"18080"'
printf '%s' "$both" | grep -Fq '"published":"18082"'
printf '%s' "$both" | grep -Fq '"GSM_LISTEN":":8082"'
unset base both
echo 'PASS actual Compose Web-only and explicit API overlay / 实际 Compose 默认仅 Web 与双端口合并'

docker run -d --name "$name" --label "com.gsm-system.web-fixture=$run_id" --network none \
  -e GSM_LISTEN=127.0.0.1:8082 -e GSM_WEB_LISTEN=:18082 -e GSM_WEB_ENABLED=true \
  -e GSM_API_TOKEN=web-fixture-token "$image" >/dev/null
i=0
until docker exec "$name" curl --connect-timeout 3 --max-time 10 -fsS http://127.0.0.1:18082/web-meta.json >/dev/null 2>&1; do
  i=$((i+1)); [ "$i" -lt 35 ] || { docker logs "$name"; exit 1; }; sleep 1
done
docker exec "$name" curl --connect-timeout 3 --max-time 10 -fsS http://127.0.0.1:18082/ | grep -qi '<!doctype html'
meta=$(docker exec "$name" curl --connect-timeout 3 --max-time 10 -fsS http://127.0.0.1:18082/web-meta.json)
printf '%s' "$meta" | grep -Fq '"token_required":true'
if printf '%s' "$meta" | grep -Fq web-fixture-token; then echo 'FAIL token leaked in metadata'; exit 1; fi
for port in 18082 8082; do
  code=$(docker exec "$name" curl --connect-timeout 3 --max-time 10 -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/api/v1/cell")
  [ "$code" = 401 ]
  docker exec "$name" curl --connect-timeout 3 --max-time 10 -fsS -H 'Authorization: Bearer web-fixture-token' \
    "http://127.0.0.1:$port/api/v1/cell" | grep -Fq '"state":"stopped"'
done
code=$(docker exec "$name" curl --connect-timeout 3 --max-time 10 -sS -o /dev/null -w '%{http_code}' \
  -X POST -H 'Origin: https://untrusted.invalid' -H 'Authorization: Bearer web-fixture-token' \
  -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:18082/api/v1/cell)
[ "$code" = 403 ]
docker exec "$name" /usr/local/bin/gsm-system --healthcheck
echo 'PASS Web assets, secret-free metadata, identical auth, cross-origin mutation rejection and dual-listener health / 页面、元数据、同鉴权、跨源写拒绝与双监听健康通过'
