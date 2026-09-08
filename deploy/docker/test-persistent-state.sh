#!/bin/sh
# Isolated filesystem/SQLite regression tests; no services, containers or RF.
# 隔离文件系统和 SQLite 回归测试；不启动服务、容器或射频。
set -eu
. "$(dirname "$0")/persistent-state.sh"
command -v sqlite3 >/dev/null || { echo 'sqlite3 is required' >&2; exit 1; }
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM
seed="$workspace/seed.sql"
printf "CREATE TABLE sample(value TEXT); INSERT INTO sample VALUES ('seed');\n" > "$seed"

mkdir -p "$workspace/native" "$workspace/volume"
persist_directory "$workspace/native" "$workspace/volume/state"
initialize_sqlite "$workspace/native/config.db" "$seed" sql
test -L "$workspace/native"
sqlite3 "$workspace/native/config.db" "UPDATE sample SET value='kept';"
persist_directory "$workspace/native" "$workspace/volume/state"
initialize_sqlite "$workspace/native/config.db" "$seed" sql
test "$(sqlite3 "$workspace/native/config.db" 'SELECT value FROM sample;')" = kept
echo 'PASS: first boot and restart preserve data / 首启与重启保留数据'

# Simulate a fresh container with a different seed and the same mounted volume.
mkdir "$workspace/recreated"
sqlite3 "$workspace/recreated/config.db" < "$seed"
printf 'stale unrelated WAL' > "$workspace/recreated/config.db-wal"
printf 'stale unrelated SHM' > "$workspace/recreated/config.db-shm"
persist_directory "$workspace/recreated" "$workspace/volume/state"
initialize_sqlite "$workspace/recreated/config.db" "$seed" sql
test "$(sqlite3 "$workspace/recreated/config.db" 'SELECT value FROM sample;')" = kept
test ! -e "$workspace/volume/state/config.db-wal"
test ! -e "$workspace/volume/state/config.db-shm"
test -f "$workspace/recreated.pre-persistence/config.db"
echo 'PASS: recreate keeps volume data and original backup / 重建保留卷及旧库备份'

mkdir "$workspace/old-native"
sqlite3 "$workspace/old-native/registry.db" < "$seed"
sqlite3 "$workspace/old-native/registry.db" "UPDATE sample SET value='migrated';"
persist_directory "$workspace/old-native" "$workspace/volume/migrated"
initialize_sqlite "$workspace/old-native/registry.db" "$workspace/recreated.pre-persistence/config.db" db
test "$(sqlite3 "$workspace/old-native/registry.db" 'SELECT value FROM sample;')" = migrated
initialize_sqlite "$workspace/volume/new-registry.db" "$workspace/old-native/registry.db" db
test "$(sqlite3 "$workspace/volume/new-registry.db" 'SELECT value FROM sample;')" = migrated
echo 'PASS: native database migration and binary seeds / 原生库迁移及二进制种子'

# CDR directory migration keeps volume history and only fills missing files.
mkdir "$workspace/old-cdr" "$workspace/volume/cdr"
printf 'container-copy\n' >"$workspace/old-cdr/Master.csv"
printf 'old-extra\n' >"$workspace/old-cdr/queue_log"
printf 'volume-history\n' >"$workspace/volume/cdr/Master.csv"
persist_directory "$workspace/old-cdr" "$workspace/volume/cdr"
test -L "$workspace/old-cdr"
test "$(cat "$workspace/old-cdr/Master.csv")" = volume-history
test "$(cat "$workspace/old-cdr/queue_log")" = old-extra
test "$(cat "$workspace/old-cdr.pre-persistence/Master.csv")" = container-copy
persist_directory "$workspace/old-cdr" "$workspace/volume/cdr"
echo 'PASS: CDR migration preserves volume history / CDR 迁移保留卷内历史'

ln -s "$workspace/missing" "$workspace/broken"
if (set -e; persist_directory "$workspace/broken" "$workspace/volume/state"); then
  echo 'FAIL: broken link was accepted' >&2; exit 1
fi
mkdir "$workspace/ambiguous" "$workspace/ambiguous.pre-persistence"
if (set -e; persist_directory "$workspace/ambiguous" "$workspace/volume/ambiguous"); then
  echo 'FAIL: interrupted migration was accepted' >&2; exit 1
fi
printf 'not a database' > "$workspace/volume/corrupt.db"
if (set -e; initialize_sqlite "$workspace/volume/corrupt.db" "$seed" sql); then
  echo 'FAIL: corrupted database was accepted' >&2; exit 1
fi
test "$(cat "$workspace/volume/corrupt.db")" = 'not a database'
: > "$workspace/volume/empty.db"
if (set -e; initialize_sqlite "$workspace/volume/empty.db" "$seed" sql); then
  echo 'FAIL: empty existing database was accepted' >&2; exit 1
fi
test ! -s "$workspace/volume/empty.db"
echo 'PASS: invalid existing state fails without overwrite / 异常现存状态停止启动且不覆盖'

# Welcome migration is idempotent and never enables disabled messages.
welcome="$workspace/welcome.db"
sqlite3 "$welcome" "CREATE TABLE CONFIG(KEYSTRING TEXT PRIMARY KEY, VALUESTRING TEXT);
INSERT INTO CONFIG VALUES('Control.LUR.OpenRegistration.Message','Welcome to addx. Your IMSI is ');
INSERT INTO CONFIG VALUES('Control.LUR.NormalRegistration.Message','');"
migrate_welcome_defaults "$welcome"
migrate_welcome_defaults "$welcome"
test "$(sqlite3 "$welcome" "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='Control.LUR.OpenRegistration.Message';")" = 'Welcome to addx. Reply to 101 with a 7-10 digit number, e.g. 10000001. Send info to 411 to check your number. '
test -z "$(sqlite3 "$welcome" "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='Control.LUR.NormalRegistration.Message';")"
sqlite3 "$welcome" "UPDATE CONFIG SET VALUESTRING='Operator custom text';"
migrate_welcome_defaults "$welcome"
test "$(sqlite3 "$welcome" "SELECT count(*) FROM CONFIG WHERE VALUESTRING='Operator custom text';")" = 2
echo 'PASS: welcome defaults migrate once, custom and disabled messages preserved / 欢迎默认迁移且保留自定义与禁用设置'

# Fresh GPRS defaults are atomic; recreation preserves custom/disabled state.
gprs_seed="$workspace/gprs-seed.sql"
cat >"$gprs_seed" <<'SQL'
CREATE TABLE CONFIG(KEYSTRING TEXT PRIMARY KEY, VALUESTRING TEXT);
INSERT INTO CONFIG VALUES('GPRS.Enable','0');
INSERT INTO CONFIG VALUES('GPRS.Channels.Min.C0','2');
INSERT INTO CONFIG VALUES('GPRS.Channels.Min.CN','0');
INSERT INTO CONFIG VALUES('GGSN.DNS','');
SQL
GSM_GPRS_DNS=192.0.2.53 initialize_sqlite "$workspace/gprs.db" "$gprs_seed" openbts
test "$(sqlite3 "$workspace/gprs.db" "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GPRS.Enable';")" = 1
test "$(sqlite3 "$workspace/gprs.db" "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GGSN.DNS';")" = 192.0.2.53
sqlite3 "$workspace/gprs.db" "UPDATE CONFIG SET VALUESTRING='0' WHERE KEYSTRING='GPRS.Enable';"
GSM_GPRS_DNS=1.1.1.1 initialize_sqlite "$workspace/gprs.db" "$gprs_seed" openbts
test "$(sqlite3 "$workspace/gprs.db" "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GPRS.Enable';")" = 0
test "$(sqlite3 "$workspace/gprs.db" "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GGSN.DNS';")" = 192.0.2.53
for invalid in 127.0.0.11 0.0.0.0 224.0.0.1 256.1.1.1 1.2.3 1.2.3.04 '1.1.1.1;DROP'; do
  if (GSM_GPRS_DNS=$invalid initialize_sqlite "$workspace/rejected.db" "$gprs_seed" openbts); then
    echo 'invalid fresh GPRS DNS was accepted' >&2; exit 1
  fi
  test ! -e "$workspace/rejected.db"
  rm -f "$workspace/rejected.db.init-$$"
done
echo 'PASS fresh GPRS defaults and existing-state preservation / 新卷GPRS默认开启且旧配置保留'
