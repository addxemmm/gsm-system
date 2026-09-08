#!/bin/sh
# Isolated files only; no Docker/native services / 仅隔离文件测试。
set -eu
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$script_dir/timezone.sh"
temp=$(mktemp -d)
cleanup() {
  rm -f -- "$temp/localtime" "$temp/timezone" "$temp/zones/Asia/Shanghai" "$temp/zones/UTC"
  rmdir -- "$temp/zones/Asia" "$temp/zones" "$temp"
}
trap cleanup EXIT HUP INT TERM
mkdir -p "$temp/zones/Asia"
printf shanghai > "$temp/zones/Asia/Shanghai"
printf utc > "$temp/zones/UTC"
configure_timezone Asia/Shanghai "$temp/zones" "$temp/localtime" "$temp/timezone"
cmp -s "$temp/localtime" "$temp/zones/Asia/Shanghai"
# Git Bash emulates symlinks as files / Git Bash 可能将符号链接模拟为文件。
case "$(uname -s)" in MINGW*|MSYS*) ;; *) test "$(readlink "$temp/localtime")" = "$temp/zones/Asia/Shanghai" ;; esac
test "$(cat "$temp/timezone")" = Asia/Shanghai
configure_timezone UTC "$temp/zones" "$temp/localtime" "$temp/timezone"
cmp -s "$temp/localtime" "$temp/zones/UTC"
test "$(cat "$temp/timezone")" = UTC
for zone in missing/zone ../UTC /etc/localtime Local :UTC ''; do
  if configure_timezone "$zone" "$temp/zones" "$temp/localtime" "$temp/timezone" 2>/dev/null; then
    echo "invalid timezone accepted: $zone" >&2
    exit 1
  fi
  test "$(cat "$temp/timezone")" = UTC
done
echo 'timezone fixture tests passed / 时区隔离测试通过'
