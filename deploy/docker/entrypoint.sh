#!/bin/sh
# entrypoint: init DBs (fresh image has none baked in), reset volatile
# registry to clean seeds (stateless start-clean, legacy v1.3 behavior),
# then exec the Go API server. No Python, no Flask, no gunicorn.
set -eu
mkdir -p /data/conf /data/log /etc/OpenBTS /var/run /var/lib/asterisk/sqlite3dir
# One-time DB creation from the freshly built example schema.
if [ ! -f /etc/OpenBTS/OpenBTS.db ] && [ -f /app/seeds/OpenBTS.example.sql ]; then
  sqlite3 /etc/OpenBTS/OpenBTS.db < /app/seeds/OpenBTS.example.sql
fi

# Volatile subscriber registry always starts clean (documented in RULES.md).
if [ -f /OpenBTS/TMSITable_init.db ]; then
  cp -f /OpenBTS/TMSITable_init.db /var/run/TMSITable.db || true
fi
if [ -f /OpenBTS/sqlite3_init.db ]; then
  cp -f /OpenBTS/sqlite3_init.db /var/lib/asterisk/sqlite3dir/sqlite3.db || true
fi

# Asterisk runs as root:www-data (asterisk.conf runuser/rungroup): state dirs
# and the registry DB must be group-writable, AFTER the seed copy above.
mkdir -p /var/lib/asterisk /var/spool/asterisk /var/log/asterisk /var/run/asterisk
chown -R root:www-data /var/lib/asterisk /var/spool/asterisk /var/log/asterisk /var/run/asterisk 2>/dev/null || true
chmod -R g+rwX /var/lib/asterisk /var/spool/asterisk /var/log/asterisk /var/run/asterisk
exec /usr/local/bin/gsm-system
