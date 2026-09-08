#!/bin/sh
# Compile the actual pinned/patched optional-IE loop, without services or RF.
# 原样提取生产解析循环，以边界检查适配器隔离运行；不启动服务、数据库或射频。
set -eu
source_file=${1:?usage: test-gmm-optional-ies.sh PATH/SGSNGGSN/GPRSL3Messages.cpp}
test_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM
awk '
  /^void GMMAttach::gmParseIEs\(/ { copying=1; found++ }
  copying { print }
  copying && /^}/ { copying=0 }
  END { if (found != 1 || copying) exit 1 }
' "$source_file" > "$workspace/gmm-optional.inc"
${CXX:-c++} -std=c++14 -Wall -Wextra -Werror -I "$workspace" \
  "$test_dir/gmm-optional-ies.cpp" -o "$workspace/gmm-optional-ies"
"$workspace/gmm-optional-ies"
