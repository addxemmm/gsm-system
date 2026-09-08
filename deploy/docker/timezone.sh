#!/bin/sh
# Select a display timezone, never adjust the kernel clock.
# 只配置展示时区，不修改系统时钟；路径参数用于隔离文件测试。
configure_timezone() {
  zone=$1
  zoneinfo=$2
  localtime=$3
  timezone_file=$4
  case "$zone" in
    ''|Local|/*|*..*|*:*|*\\*)
      echo "Invalid timezone/TZ: $zone (use an IANA timezone name)" >&2
      return 1
      ;;
  esac
  if [ ! -f "$zoneinfo/$zone" ]; then
    echo "Invalid timezone/TZ: $zone (not installed in $zoneinfo)" >&2
    return 1
  fi
  ln -snf "$zoneinfo/$zone" "$localtime" || return 1
  printf '%s\n' "$zone" > "$timezone_file"
}
