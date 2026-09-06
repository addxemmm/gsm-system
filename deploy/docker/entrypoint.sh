#!/bin/sh
# Initialize persistent configuration/subscribers only once; TMSI remains volatile.
# 配置和签约库仅首次初始化并持久化，TMSI 仍为易失运行态。
set -eu
. /usr/local/lib/gsm-persistent-state.sh
data_dir=${GSM_DATA_DIR:-/data}
mkdir -p "$data_dir/conf" "$data_dir/log" /etc /var/run /var/lib/asterisk
persist_directory /etc/OpenBTS "$data_dir/state/OpenBTS"
persist_directory /var/lib/asterisk/sqlite3dir "$data_dir/state/asterisk"
# smqueue writes CDRs here; missing dir kills it at boot (seen live).
mkdir -p /var/lib/OpenBTS
initialize_sqlite /etc/OpenBTS/OpenBTS.db /app/seeds/OpenBTS.example.sql sql
initialize_sqlite /var/lib/asterisk/sqlite3dir/sqlite3.db /OpenBTS/sqlite3_init.db db
for database in /etc/OpenBTS/sipauthserve.db /etc/OpenBTS/smqueue.db; do
  if [ -e "$database" ]; then
    check_sqlite "$database"
  fi
done

# Only the volatile TMSI table starts clean; never overwrite subscriber records.
# 仅重置易失 TMSI 表，保留持久签约记录。
if [ -f /OpenBTS/TMSITable_init.db ]; then
  cp -f /OpenBTS/TMSITable_init.db /var/run/TMSITable.db
fi

# Asterisk runs as root:www-data (asterisk.conf runuser/rungroup): state dirs
# and the registry DB must be group-writable, AFTER the seed copy above.
mkdir -p /var/lib/asterisk /var/spool/asterisk /var/log/asterisk /var/run/asterisk
chown -R root:www-data /var/lib/asterisk /var/spool/asterisk /var/log/asterisk /var/run/asterisk 2>/dev/null || true
chmod -R g+rwX /var/lib/asterisk /var/spool/asterisk /var/log/asterisk /var/run/asterisk
chown -R root:www-data "$data_dir/state/asterisk"
chmod -R g+rwX "$data_dir/state/asterisk"
exec /usr/local/bin/gsm-system
