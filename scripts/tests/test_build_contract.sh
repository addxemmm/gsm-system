#!/bin/sh
# Offline 2.1 build/release contract; no Docker, network, services or RF.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
fail() { echo "FAIL [BUILD-CONTRACT] $1" >&2; exit 1; }

version=$(tr -d '[:space:]' <VERSION)
printf '%s\n' "$version" | grep -Eq '^2\.1\.(0|[1-9][0-9]*)$' || fail 'VERSION must stay on the explicitly supported 2.1 release line'
[ "$(find deploy/docker -maxdepth 1 -type f -name 'Dockerfile*' | wc -l | tr -d '[:space:]')" = 1 ] || \
  fail 'multiple Dockerfiles remain'
[ "$(find deploy/docker -maxdepth 1 -type f -name 'docker-compose*.yml' | wc -l | tr -d '[:space:]')" = 2 ] || \
  fail 'expected one production Compose plus optional API overlay'

grep -F 'org.opencontainers.image.version="${VERSION}"' deploy/docker/Dockerfile >/dev/null || fail 'OCI version label missing'
grep -F 'org.opencontainers.image.revision="${REVISION}"' deploy/docker/Dockerfile >/dev/null || fail 'OCI revision label missing'
grep -F -- '-X main.version=${VERSION} -X main.revision=${REVISION}' deploy/docker/Dockerfile >/dev/null || fail 'Docker Go ldflags missing'
grep -F 'COPY cmd/ ./cmd/' deploy/docker/Dockerfile >/dev/null || fail 'minimal cmd COPY missing'
grep -F 'COPY internal/ ./internal/' deploy/docker/Dockerfile >/dev/null || fail 'minimal internal COPY missing'
if grep -Eq '^COPY[[:space:]]+\.[[:space:]]+\.' deploy/docker/Dockerfile; then fail 'broad Go-stage COPY remains'; fi
if grep -F 'run.so' deploy/docker/Dockerfile >/dev/null; then fail 'legacy run.so is copied'; fi
grep -F 'COPY deploy/docker/sip-contact-acl.conf /etc/asterisk/sip-contact-acl.conf' deploy/docker/Dockerfile >/dev/null || fail 'local SIP contact ACL is not shipped'
grep -F '#include sip-contact-acl.conf' deploy/docker/Dockerfile >/dev/null || fail 'local SIP contact ACL is not included'
grep -Fx 'contactdeny=0.0.0.0/0.0.0.0' deploy/docker/sip-contact-acl.conf >/dev/null || fail 'IPv4 contact deny missing'
grep -Fx 'contactdeny=::/0' deploy/docker/sip-contact-acl.conf >/dev/null || fail 'IPv6 contact deny missing'
grep -Fx 'contactpermit=127.0.0.1/255.255.255.255' deploy/docker/sip-contact-acl.conf >/dev/null || fail 'OpenBTS loopback contact permit missing'
[ "$(grep -c '^contactpermit=' deploy/docker/sip-contact-acl.conf)" -eq 1 ] || fail 'unexpected contact allow expansion'
grep -F '0003-smqueue-keyed-sms-observation.patch' deploy/docker/Dockerfile >/dev/null || fail 'keyed native SMS observation patch is not applied'
grep -F 'GSM_SMS_V1' compat/patches/0003-smqueue-keyed-sms-observation.patch >/dev/null || fail 'keyed native SMS observation format missing'
grep -F '0006-smqueue-ucs2-decode.patch' deploy/docker/Dockerfile >/dev/null || fail 'native UCS-2 decode patch is not applied'
grep -F 'test-sms-ucs2.sh SMS/SMSMessages.cpp' deploy/docker/Dockerfile >/dev/null || fail 'native UCS-2 regression test is not run'
grep -F 'patch -p1 < /app/compat/patches/0005-uhd-rx-timeout-retry.patch' deploy/docker/Dockerfile >/dev/null || fail 'UHD RX timeout retry patch is not applied'
grep -F 'sh /app/compat/tests/test-uhd-rx-timeout.sh Transceiver52M/UHDDevice.cpp' deploy/docker/Dockerfile >/dev/null || fail 'patched UHD RX timeout harness is not run'
[ -f compat/patches/0005-uhd-rx-timeout-retry.patch ] || fail 'UHD RX timeout retry patch missing'
[ -f compat/tests/test-uhd-rx-timeout.sh ] || fail 'UHD RX timeout harness missing'
grep -F 'patch -p1 < /app/compat/patches/0007-gprs-pre-imsi-ccch-assignment.patch' deploy/docker/Dockerfile >/dev/null || fail 'GPRS pre-IMSI assignment patch missing'
grep -F 'sh /app/compat/tests/test-gprs-pre-imsi.sh .' deploy/docker/Dockerfile >/dev/null || fail 'GPRS pre-IMSI regression is not run'
[ -f compat/tests/gprs-pre-imsi.cpp ] || fail 'GPRS pre-IMSI native test missing'

grep -F 'image: "${GSM_IMAGE:-gsm-system:2.1}"' deploy/docker/docker-compose.yml >/dev/null || fail '2.1 runtime image default missing'
grep -F 'com.gsm-system.managed: "true"' deploy/docker/docker-compose.yml >/dev/null || fail 'managed GSM cleanup label missing'
grep -F 'RUNTIME_IMAGE=gsm-system:2.1' scripts/deploy_to_ubuntu.sh >/dev/null || fail 'stable deploy image missing'
grep -F "label=org.opencontainers.image.title=gsm-system" scripts/deploy_to_ubuntu.sh >/dev/null || fail 'GSM-scoped image cleanup missing'
grep -F 'docker builder prune --all --force' scripts/deploy_to_ubuntu.sh >/dev/null || fail 'post-validation build-cache cleanup missing'
grep -F 'docker exec "$HEALTH_CONTAINER" /usr/local/bin/gsm-system --healthcheck' scripts/deploy_to_ubuntu.sh >/dev/null || fail 'binary health cleanup gate missing'
if grep -Eq 'gsm-system:2\.1\.0-[0-9a-f]|GSM_IMMUTABLE_IMAGE' Makefile scripts/deploy_to_ubuntu.sh scripts/deploy_from_windows.ps1; then
  fail 'revision-suffixed runtime image scheme remains in release scripts'
fi
grep -F 'container_name: gsmsystem-uhd4' deploy/docker/docker-compose.yml >/dev/null || fail 'collision-free container name missing'
grep -F 'external: true' deploy/docker/docker-compose.yml >/dev/null || fail 'data volume is not external'
grep -F 'test: ["CMD", "/usr/local/bin/gsm-system", "--healthcheck"]' deploy/docker/docker-compose.yml >/dev/null || fail 'state-aware Go health probe missing'
grep -F 'migrate_welcome_defaults /etc/OpenBTS/OpenBTS.db' deploy/docker/entrypoint.sh >/dev/null || fail 'welcome migration missing'
grep -F 'docker_gsm-data' deploy/docker/docker-compose.yml >/dev/null || fail 'existing data volume name missing'
if grep -F 'network_mode: host' deploy/docker/docker-compose.yml >/dev/null; then
  fail 'host networking remains enabled'
fi
grep -F '${GSM_BIND_ADDRESS:-0.0.0.0}:${GSM_WEB_PORT:-8080}:8080/tcp' deploy/docker/docker-compose.yml >/dev/null || \
  fail 'default Web publication missing'
grep -F 'GSM_LISTEN: "127.0.0.1:8082"' deploy/docker/docker-compose.yml >/dev/null || fail 'default API is not loopback-only'
grep -F '${GSM_API_PORT:-8082}:8082/tcp' deploy/docker/docker-compose.api.yml >/dev/null || fail 'optional API publication missing'
grep -F 'GSM_LISTEN: ":8082"' deploy/docker/docker-compose.api.yml >/dev/null || fail 'API overlay listener missing'
published_ports=$(awk '
  /^    ports:$/ { in_ports=1; next }
  in_ports && /^    [A-Za-z_][A-Za-z0-9_-]*:/ { in_ports=0 }
  in_ports && /^      - / { count++ }
  END { print count + 0 }
' deploy/docker/docker-compose.yml)
[ "$published_ports" -eq 1 ] || fail "expected exactly one published port, got $published_ports"
grep -F 'net.ipv4.ip_forward: "1"' deploy/docker/docker-compose.yml >/dev/null || \
  fail 'container IPv4 forwarding is not explicit'
grep -F '${GSM_BRIDGE_SUBNET:-172.31.240.0/24}' deploy/docker/docker-compose.yml >/dev/null || \
  fail 'dedicated non-overlapping bridge subnet missing'
if grep -Eq '(^|[^0-9])(506[0-4]|49300|16484|16581|20000|22000)([^0-9]|$).*:' deploy/docker/docker-compose.yml; then
  fail 'internal SIP/RTP/CLI port is published'
fi

if rg -n --glob '!test_build_contract.sh' 'docker-compose\.uhd4|Dockerfile\.uhd4|gsmsystem-uhd4:test' \
  Makefile .github/workflows/ci.yml deploy/docker scripts >/dev/null; then
  fail 'retired experimental build path is still referenced'
fi

for obsolete in \
  configs/seeds/run.so \
  gsmsystem/run.py gsmsystem/run_c.py gsmsystem/setup.py \
  gsmsystem/run.sh gsmsystem/stop.sh gsmsystem/stop_systemctl.sh \
  gsmsystem/supervisord.conf \
  gsmsystem_v1.3
do
  [ ! -e "$obsolete" ] || fail "obsolete runtime remains: $obsolete"
done

for pair in \
  cppzmq:76bf169fd67b8e99c1b0e6490029d9cd5ef97666 \
  liba53:27354560dc7b554e03d40a520d41290e731193b6 \
  libcoredumper:7527fb3804927c7fdc72ff5139a2cdea3db4d59a \
  openbts:7766ef94f2d885c197430e74a89f02740f0c04e6 \
  smqueue:e168a262db311231c51cf7295f9bd1f440567485 \
  subscriberRegistry:c65b5d59f744a8df5f3395e217f2598f53e9fe65 \
  uhd4:d21735d543d5a3c265507965c7bb6c9e9df95fcd
do
  revision=${pair#*:}
  grep -F "$revision" scripts/prefetch_vendor.sh >/dev/null || fail "prefetch lock missing: $pair"
  grep -F "$revision" deploy/docker/Dockerfile >/dev/null || fail "Docker manifest check missing: $pair"
done
archive_sha=c7dc2f0ee2b00a3192bd1fa595d175ce6ecf6c5499220ba1232fa2fb4cc3774d
grep -F "$archive_sha" scripts/prefetch_vendor.sh >/dev/null || fail 'prefetch archive hash missing'
grep -F "$archive_sha" deploy/docker/Dockerfile >/dev/null || fail 'Docker archive hash missing'

command -v go >/dev/null 2>&1 || fail 'go is required'
TMP=${TMPDIR:-/tmp}/gsm-build-contract-$$
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
mkdir -p "$TMP"
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=$version -X main.revision=0123456789ab" \
  -o "$TMP/gsm-system" ./cmd/server
[ "$("$TMP/gsm-system" --version)" = "gsm-system $version (0123456789ab)" ] || \
  fail 'Go binary version metadata mismatch'

echo 'PASS test_build_contract.sh / 2.1 构建发布契约通过'
