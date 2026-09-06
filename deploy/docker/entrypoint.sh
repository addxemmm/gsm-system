#!/bin/sh
# entrypoint: rebuild volatile DBs from seeds (stateless start-clean),
# then exec the Go API server. Legacy v1.3 stop.sh reset logic, moved to boot.
set -eu
mkdir -p /data/conf /data/log
if [ -f /OpenBTS/TMSITable_init.db ]; then
  cp -f /OpenBTS/TMSITable_init.db /var/run/TMSITable.db || true
fi
if [ -f /OpenBTS/sqlite3_init.db ]; then
  mkdir -p /var/lib/asterisk/sqlite3dir
  cp -f /OpenBTS/sqlite3_init.db /var/lib/asterisk/sqlite3dir/sqlite3.db || true
fi
exec /usr/local/bin/gsm-system
