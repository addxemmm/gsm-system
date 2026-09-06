#!/bin/sh
# Isolated filesystem/SQLite regression tests; no services, containers or RF.
# 隔离文件系统和 SQLite 回归测试；不启动服务、容器或射频。
set -eu
. "$(dirname "$0")/persistent-state.sh"
command -v sqlite3 >/dev/null || { echo 'sqlite3 is required' >&2; exit 1; }
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM
seed="$workspace/seed.sql"
printf 'CREATE TABLE sample(value TEXT); INSERT INTO sample VALUES ("seed");\n' > "$seed"

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
