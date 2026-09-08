#!/bin/sh
# Test extracted production SMS methods without RF/services.
# 无射频/服务测试；反向应用补丁验证原版本确实拒绝中文 DCS。
set -eu
source_file=${1:?usage: test-sms-ucs2.sh PATH/SMS/SMSMessages.cpp}
test_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
patch_file=$(CDPATH= cd -- "$test_dir/../patches" && pwd)/0006-smqueue-ucs2-decode.patch
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM
compile_case() {
    input=$1
    output=$2
    mkdir -p "$output"
    awk '
      /^(std::string TLUserData::decode|void TLUserData::parse|void TLUserData::write)\(/ { copying=1; found++ }
      copying { print }
      copying && /^}/ { copying=0 }
      END { if (found != 3 || copying) exit 1 }
    ' "$input" > "$output/sms-ucs2-methods.inc"
    ${CXX:-c++} -std=gnu++11 -Wall -Wextra -Werror -I "$output" \
        "$test_dir/sms-ucs2.cpp" -o "$output/sms-ucs2"
}
compile_case "$source_file" "$workspace/patched"
"$workspace/patched/sms-ucs2"
mkdir -p "$workspace/original/SMS"
cp "$source_file" "$workspace/original/SMS/SMSMessages.cpp"
(CDPATH= cd -- "$workspace/original"; patch -s -R -p1 < "$patch_file")
compile_case "$workspace/original/SMS/SMSMessages.cpp" "$workspace/original-test"
if "$workspace/original-test/sms-ucs2" >"$workspace/original.log" 2>&1; then
    echo 'FAIL: original negative control unexpectedly decoded UCS-2' >&2
    exit 1
fi
grep -F 'EXPECTED-NEGATIVE: original rejects DCS 0x08' "$workspace/original.log" >/dev/null
echo 'PASS: original negative control rejects DCS 0x08'
