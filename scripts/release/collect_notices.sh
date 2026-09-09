#!/bin/sh
# Keep upstream paths and notice contents verbatim / 保留上游路径和原始许可内容。
set -eu
SOURCE=${1:?source tree required}
DESTINATION=${2:?absolute destination required}
case "$DESTINATION" in /*) ;; *) echo 'absolute destination required' >&2; exit 1 ;; esac
mkdir -p "$DESTINATION"
cd "$SOURCE"
find . -type f \( -iname 'license*' -o -iname 'copying*' -o -iname 'notice*' -o -iname 'copyright*' \) \
  -exec cp --parents -- '{}' "$DESTINATION/" \;
test -n "$(find "$DESTINATION" -type f -print -quit)"
