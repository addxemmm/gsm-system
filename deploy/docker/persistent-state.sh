#!/bin/sh
# Keep native SQLite directories on the /data volume, including WAL files.
# 原生 SQLite 目录（含 WAL 文件）存放于 /data 卷；已有数据优先，不覆盖。
# Sourced by entrypoint and isolated filesystem tests; no startup side effects.

persist_directory() {
  source_dir=$1
  target_dir=$2
  if [ -L "$source_dir" ]; then
    if [ "$(readlink "$source_dir")" != "$target_dir" ] || [ ! -d "$target_dir" ]; then
      echo "Unexpected or broken state link: $source_dir" >&2
      return 1
    fi
    return 0
  fi
  if [ -e "$source_dir" ] && [ ! -d "$source_dir" ]; then
    echo "State path is not a directory: $source_dir" >&2
    return 1
  fi
  # Refuse ambiguous interrupted migrations, retaining both copies for inspection.
  # 中断迁移留下备份时停止，保留现场，不猜测哪一份为有效数据。
  if [ -e "$source_dir.pre-persistence" ]; then
    echo "Previous state migration requires inspection: $source_dir.pre-persistence" >&2
    return 1
  fi
  mkdir -p "$target_dir" || return 1
  if [ -d "$source_dir" ]; then
    # Existing volume contents always win; never reset a registry to image seeds.
    # 已有卷中的文件优先；仅补齐缺失文件，不用镜像种子重置签约库。
    # SQLite DB/WAL/SHM/journal are a bundle. Never combine an existing target
    # database with sidecars belonging to another source database.
    # SQLite 主库及其旁文件须成组迁移，不把旧 WAL 合并到已有目标库。
    for entry in "$source_dir"/* "$source_dir"/.[!.]* "$source_dir"/..?*; do
      [ -e "$entry" ] || [ -L "$entry" ] || continue
      name=${entry##*/}
      case "$name" in
        *.db-wal|*.db-shm|*.db-journal)
          base=${entry%-*}
          [ -f "$base" ] || { echo "Orphan SQLite sidecar: $entry" >&2; return 1; }
          ;;
        *.db)
          if [ ! -e "$target_dir/$name" ]; then
            for suffix in -wal -shm -journal; do
              [ ! -e "$target_dir/$name$suffix" ] || {
                echo "Orphan target SQLite sidecar: $target_dir/$name$suffix" >&2
                return 1
              }
            done
            cp -a "$entry" "$target_dir/$name" || return 1
            for suffix in -wal -shm -journal; do
              if [ -f "$entry$suffix" ]; then
                cp -a "$entry$suffix" "$target_dir/$name$suffix" || return 1
              fi
            done
          fi
          ;;
        *) cp -an "$entry" "$target_dir/" || return 1 ;;
      esac
    done
    mv "$source_dir" "$source_dir.pre-persistence" || return 1
  fi
  ln -s "$target_dir" "$source_dir"
}

initialize_sqlite() {
  database=$1
  seed=$2
  seed_type=$3
  if [ ! -e "$database" ]; then
    if [ ! -f "$seed" ]; then
      echo "Missing database seed: $seed" >&2
      return 1
    fi
    temporary="$database.init-$$"
    # A restarted container reuses PID 1. Discard only this unpublished seed
    # bundle so correcting an invalid setting can recover without manual cleanup.
    # 容器重启可能复用 PID；仅清理该未发布种子及旁文件，修正配置即可重试。
    clear_sqlite_seed "$temporary" || return 1
    if [ "$seed_type" = sql ] || [ "$seed_type" = openbts ]; then
      sqlite3 -bail "$temporary" < "$seed" || { clear_sqlite_seed "$temporary"; return 1; }
      if [ "$seed_type" = openbts ]; then
        seed_gprs_defaults "$temporary" || { clear_sqlite_seed "$temporary"; return 1; }
      fi
    else
      cp "$seed" "$temporary" || { clear_sqlite_seed "$temporary"; return 1; }
    fi
    if [ "$(sqlite3 -bail "$temporary" 'PRAGMA quick_check;')" != ok ]; then
      echo "Database seed integrity check failed: $seed" >&2
      clear_sqlite_seed "$temporary"
      return 1
    fi
    mv "$temporary" "$database" || return 1
  fi
  check_sqlite "$database"
}

clear_sqlite_seed() {
  rm -f -- "$1" "$1-journal" "$1-wal" "$1-shm"
}

# Called only on the unpublished fresh OpenBTS database. Existing operator
# settings (including explicitly disabled GPRS) are never overwritten.
# 仅对尚未发布的新 OpenBTS 库应用默认值，不覆盖旧卷的自定义或关闭设置。
seed_gprs_defaults() {
  dns=${GSM_GPRS_DNS:-1.1.1.1}
  case "$dns" in ''|*[!0-9.]*) echo 'GSM_GPRS_DNS must be an upstream IPv4 address' >&2; return 1 ;; esac
  if ! printf '%s\n' "$dns" | awk -F. '
    NF != 4 { exit 1 }
    { for (i=1; i<=4; i++) if ($i !~ /^[0-9]+$/ || $i > 255 || (length($i)>1 && substr($i,1,1)=="0")) exit 1 }
    $1==0 || $1==127 || $1>=224 || ($1==169 && $2==254) { exit 1 }
  '; then
    echo 'GSM_GPRS_DNS must be a non-loopback unicast upstream IPv4 address' >&2
    return 1
  fi
  sqlite3 -bail "$1" "BEGIN IMMEDIATE;
UPDATE CONFIG SET VALUESTRING='1' WHERE KEYSTRING='GPRS.Enable';
UPDATE CONFIG SET VALUESTRING='2' WHERE KEYSTRING='GPRS.Channels.Min.C0';
UPDATE CONFIG SET VALUESTRING='0' WHERE KEYSTRING='GPRS.Channels.Min.CN';
UPDATE CONFIG SET VALUESTRING='$dns' WHERE KEYSTRING='GGSN.DNS';
COMMIT;"
}

check_sqlite() {
  database=$1
  # Fail rather than erase a damaged or empty existing database.
  # 现有数据库为空或损坏时停止启动，保留原文件供排查。
  if [ ! -s "$database" ] || [ "$(sqlite3 -bail "$database" 'PRAGMA quick_check;')" != ok ]; then
    echo "Persistent database integrity check failed: $database" >&2
    return 1
  fi
}

# Run only before native processes start. Replace known legacy defaults, not
# operator-written messages or an intentionally blank (disabled) message.
# 仅原生进程启动前迁移已知旧默认；保留自定义文案及留空禁用设置。
migrate_welcome_defaults() {
  sqlite3 -bail "$1" <<'SQL'
.timeout 5000
BEGIN IMMEDIATE;
UPDATE CONFIG SET VALUESTRING='Welcome to addx. Reply to 101 with a 7-10 digit number, e.g. 10000001. Send info to 411 to check your number. '
WHERE KEYSTRING IN ('Control.LUR.OpenRegistration.Message','Control.LUR.NormalRegistration.Message')
AND VALUESTRING IN ('Welcome to addx. Your IMSI is ', 'Welcome to the test network.  Your IMSI is ');
COMMIT;
SQL
}
