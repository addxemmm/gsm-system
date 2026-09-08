#!/bin/sh
# Isolated Asterisk caller-identity test. No OpenBTS process, RF device, host
# network, or production database is used.
# Asterisk 主叫身份隔离测试：不启动 OpenBTS/射频，不使用主机网络或生产库。
set -eu

IMAGE=${1:-gsm-system:2.1}
LABEL=com.addx.gsm-system.callerid-test
RUN_ID_RAW=$(od -An -N8 -tx1 /dev/urandom)
RUN_ID=$(printf '%s' "$RUN_ID_RAW" | tr -d ' \n')
CONTAINER=gsm-callerid-test-$RUN_ID
CREATED=0
ASTERISK_STARTED=0

fail() { echo "FAIL [$1] $2" >&2; exit 1; }
pass() { echo "PASS [$1] $2"; }

owned() {
  [ "$(docker inspect --format "{{ index .Config.Labels \"$LABEL\" }}" "$CONTAINER" 2>/dev/null)" = "$RUN_ID" ]
}

cleanup() {
  rc=$?
  trap - EXIT HUP INT TERM
  set +e
  if [ "$rc" -ne 0 ] && [ "$CREATED" -eq 1 ] && owned; then
    docker exec "$CONTAINER" sh -c 'test ! -f /tmp/callerid-asterisk.log || cat /tmp/callerid-asterisk.log' >&2
  fi
  if [ "$ASTERISK_STARTED" -eq 1 ] && [ "$CREATED" -eq 1 ] && owned; then
    docker exec "$CONTAINER" asterisk -rx 'core stop now' >/dev/null 2>&1
  fi
  if [ "$CREATED" -eq 1 ] && owned; then
    docker rm -fv "$CONTAINER" >/dev/null || rc=1
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' HUP INT TERM

[ "$#" -le 1 ] || fail INPUT "usage: $0 [IMAGE_TAG]"
command -v docker >/dev/null 2>&1 || fail PREREQ "docker command not found"
docker image inspect "$IMAGE" >/dev/null 2>&1 || fail PREREQ "image not found: $IMAGE"
docker container inspect "$CONTAINER" >/dev/null 2>&1 && fail COLLISION "container exists: $CONTAINER"

CREATED=1
docker run -d --name "$CONTAINER" --label "$LABEL=$RUN_ID" --network none --init "$IMAGE" >/dev/null || \
  fail CREATE "failed to create isolated container"
owned || fail OWNERSHIP "container label mismatch"

# Wait for the entrypoint to initialize the container-private registry copy.
attempt=0
while [ "$attempt" -lt 30 ]; do
  if docker exec "$CONTAINER" sh -c \
      "test -s /var/lib/asterisk/sqlite3dir/sqlite3.db && sqlite3 -bail /var/lib/asterisk/sqlite3dir/sqlite3.db \"SELECT 1 FROM sqlite_master WHERE name='SIP_BUDDIES';\" | grep -qx 1"; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
[ "$attempt" -lt 30 ] || fail FIXTURE "subscriber registry was not initialized"

# Add only synthetic data to the container-private registry copy.
PEER=IMSI001010999999998
BAD_PEER=IMSI001010999999996
MAPPED=81000001
docker exec "$CONTAINER" sqlite3 -bail /var/lib/asterisk/sqlite3dir/sqlite3.db \
  "INSERT INTO SIP_BUDDIES(name,context,host,type,callerid,username) VALUES('$PEER','phones','dynamic','friend','$MAPPED','$PEER'),('$BAD_PEER','phones','dynamic','friend','bad-value','$BAD_PEER');" || \
  fail FIXTURE "failed to insert caller fixture"

docker exec -i "$CONTAINER" sh -c 'cat >>/etc/asterisk/extensions.conf' <<'EOF'
[callerid-image-test]
exten => known,1,Set(CALLERID(num)=70000001)
 same => n,Set(CALLERID(name)=OriginalKnown)
 same => n,Set(CDR(A-IMSI)=ORIGINAL)
 same => n,Set(CDR(A-Number)=70000001)
 same => n,GoSub(CallerIdentityByPeer,s,1(IMSI001010999999998))
 same => n,Set(DB(callerid-image-test/known-num)=${CALLERID(num)})
 same => n,Set(DB(callerid-image-test/known-name)=${CALLERID(name)})
 same => n,Set(DB(callerid-image-test/known-imsi)=${CDR(A-IMSI)})
 same => n,Set(DB(callerid-image-test/known-anumber)=${CDR(A-Number)})
 same => n,Hangup()

exten => unknown,1,Set(CALLERID(num)=70000002)
 same => n,Set(CALLERID(name)=OriginalUnknown)
 same => n,Set(CDR(A-IMSI)=ORIGINAL)
 same => n,Set(CDR(A-Number)=70000002)
 same => n,GoSub(CallerIdentityByPeer,s,1(IMSI001010999999997))
 same => n,Set(DB(callerid-image-test/unknown-num)=${CALLERID(num)})
 same => n,Set(DB(callerid-image-test/unknown-name)=${CALLERID(name)})
 same => n,Set(DB(callerid-image-test/unknown-imsi)=${CDR(A-IMSI)})
 same => n,Set(DB(callerid-image-test/unknown-anumber)=${CDR(A-Number)})
 same => n,Hangup()

exten => rejected,1,Set(CALLERID(num)=70000003)
 same => n,Set(CALLERID(name)=OriginalRejected)
 same => n,Set(CDR(A-IMSI)=ORIGINAL)
 same => n,GoSub(CallerIdentityByPeer,s,1(IMSI001010999999998'))
 same => n,Set(DB(callerid-image-test/rejected-num)=${CALLERID(num)})
 same => n,Set(DB(callerid-image-test/rejected-name)=${CALLERID(name)})
 same => n,Set(DB(callerid-image-test/rejected-imsi)=${CDR(A-IMSI)})
 same => n,Hangup()

exten => malformed,1,Set(CALLERID(num)=70000004)
 same => n,Set(CALLERID(name)=OriginalMalformed)
 same => n,Set(CDR(A-IMSI)=ORIGINAL)
 same => n,Set(CDR(A-Number)=70000004)
 same => n,GoSub(CallerIdentityByPeer,s,1(IMSI001010999999996))
 same => n,Set(DB(callerid-image-test/malformed-num)=${CALLERID(num)})
 same => n,Set(DB(callerid-image-test/malformed-name)=${CALLERID(name)})
 same => n,Set(DB(callerid-image-test/malformed-imsi)=${CDR(A-IMSI)})
 same => n,Set(DB(callerid-image-test/malformed-anumber)=${CDR(A-Number)})
 same => n,Hangup()
EOF
docker exec -d "$CONTAINER" sh -c 'exec asterisk -f -g >/tmp/callerid-asterisk.log 2>&1'
ASTERISK_STARTED=1

attempt=0
while [ "$attempt" -lt 30 ]; do
  if docker exec "$CONTAINER" asterisk -rx 'core waitfullybooted' >/dev/null 2>&1; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
[ "$attempt" -lt 30 ] || fail ASTERISK "Asterisk did not become ready"

dialplan_reload=$(docker exec "$CONTAINER" asterisk -rx 'dialplan reload' 2>&1) || {
  printf '%s\n' "$dialplan_reload" >&2
  fail DIALPLAN "reload failed"
}
for target in CallerIdentity CallerIdentityByPeer callerid-image-test phones from-openBTS to-openBTS; do
  docker exec "$CONTAINER" asterisk -rx "dialplan show $target" >/dev/null 2>&1 || \
    fail DIALPLAN "missing context: $target"
done
function_status=$(docker exec "$CONTAINER" asterisk -rx 'core show function ODBC_CALLERID_BY_PEER' 2>&1) || \
  fail ODBC "caller lookup function is unavailable"
printf '%s\n' "$function_status" | grep -Fq 'ODBC_CALLERID_BY_PEER' || \
  fail ODBC "caller lookup function was not registered"
channel_status=$(docker exec "$CONTAINER" asterisk -rx 'core show function CHANNEL' 2>&1) || \
  fail CHANNEL "CHANNEL function documentation is unavailable"
printf '%s\n' "$channel_status" | grep -Fq 'peername' || \
  fail CHANNEL "this Asterisk/chan_sip build does not expose CHANNEL(peername)"

run_case() {
  case_name=$1
  docker exec "$CONTAINER" asterisk -rx \
    "channel originate Local/$case_name@callerid-image-test/n application Wait 1" >/dev/null 2>&1 || \
    fail ORIGINATE "Local call failed: $case_name"
}

db_value() {
  key=$1
  output=$(docker exec "$CONTAINER" asterisk -rx "database get callerid-image-test $key" 2>&1) || return 1
  printf '%s\n' "$output" | sed -n 's/^Value: //p' | tail -n 1
}

for case_name in known unknown rejected malformed; do
  run_case "$case_name"
done

attempt=0
while [ "$attempt" -lt 20 ]; do
  if [ "$(db_value known-num || true)" = "$MAPPED" ] && \
     [ "$(db_value unknown-num || true)" = 70000002 ] && \
     [ "$(db_value rejected-num || true)" = 70000003 ] && \
     [ "$(db_value malformed-num || true)" = 70000004 ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
[ "$attempt" -lt 20 ] || fail RESULT "caller identity results were not recorded"

[ "$(db_value known-name)" = "$MAPPED" ] || fail KNOWN "mapped caller name mismatch"
[ "$(db_value known-imsi)" = "$PEER" ] || fail KNOWN "mapped IMSI mismatch"
[ "$(db_value known-anumber)" = "$MAPPED" ] || fail KNOWN "mapped CDR A-Number mismatch"
pass KNOWN "trusted peer mapped to caller ID and CDR identity"

[ "$(db_value unknown-name)" = OriginalUnknown ] || fail UNKNOWN "caller name was cleared or changed"
[ "$(db_value unknown-imsi)" = ORIGINAL ] || fail UNKNOWN "CDR IMSI was changed"
[ "$(db_value unknown-anumber)" = 70000002 ] || fail UNKNOWN "CDR A-Number was cleared or changed"
pass UNKNOWN "unknown peer preserved existing caller ID and CDR identity"

[ "$(db_value rejected-name)" = OriginalRejected ] || fail FILTER "caller name changed for rejected peer"
[ "$(db_value rejected-imsi)" = ORIGINAL ] || fail FILTER "CDR IMSI changed for rejected peer"
pass FILTER "peer containing a SQL metacharacter was rejected before lookup"

[ "$(db_value malformed-name)" = OriginalMalformed ] || fail FORMAT "caller name changed for malformed mapping"
[ "$(db_value malformed-imsi)" = ORIGINAL ] || fail FORMAT "CDR IMSI changed for malformed mapping"
[ "$(db_value malformed-anumber)" = 70000004 ] || fail FORMAT "CDR A-Number changed for malformed mapping"
pass FORMAT "malformed registry caller ID preserved the existing identity"

entry_count=$(docker exec "$CONTAINER" grep -Fc 'GoSub(CallerIdentity,s,1)' /etc/asterisk/extensions-range.conf)
[ "$entry_count" -eq 2 ] || fail ENTRY "phones/from-openBTS identity initialization count was $entry_count"
if docker exec "$CONTAINER" grep -Eq 'SIP_HEADER\(P-IMSI\).*CDR\(A-IMSI\)' /etc/asterisk/extensions-range.conf; then
  fail TRUST "dialplan still derives caller identity from P-IMSI"
fi
if docker exec "$CONTAINER" grep -Eq \
    '^[[:space:]]*same.*,[[:space:]]*Set\(CALLERID\(num\)=\$\{CDR\(A-Number\)\}\)' \
    /etc/asterisk/extensions-range.conf; then
  fail ENTRY "to-openBTS still clears caller ID unconditionally"
fi
pass ENTRY "phones and from-openBTS use trusted peer identity initialization"

for pair in 2600:Echo 2602:Milliwatt; do
  number=${pair%:*}
  application=${pair#*:}
  for context in phones default from-openBTS; do
    docker exec "$CONTAINER" asterisk -rx \
      "channel originate Local/$number@$context/n application Wait 15" >/dev/null
    attempt=0
    while [ "$attempt" -lt 10 ]; do
      channels=$(docker exec "$CONTAINER" asterisk -rx 'core show channels concise')
      if printf '%s\n' "$channels" | grep -F "!gsm-diagnostics!$number!" | \
          grep -F "!Up!$application!" >/dev/null; then
        break
      fi
      attempt=$((attempt + 1))
      sleep 1
    done
    [ "$attempt" -lt 10 ] || fail DIAGNOSTICS "$number@$context did not reach answered $application"
    # This container holds synthetic Local calls only, never live radio calls.
    docker exec "$CONTAINER" asterisk -rx 'channel request hangup all' >/dev/null
    sleep 1
    pass DIAGNOSTICS "$number@$context reached $application without SIP/RF"
  done
done

docker exec "$CONTAINER" asterisk -rx 'core stop now' >/dev/null
ASTERISK_STARTED=0
pass CALLERID "isolated caller identity checks passed for $IMAGE"
