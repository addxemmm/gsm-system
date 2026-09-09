#!/bin/sh
# Extract real methods, then prove that reversing 0007 recreates the failure.
# 提取生产实现；反向补丁负对照必须重现初始身份请求阻塞。不使用 RF/服务。
set -eu
source_root=${1:?usage: test-gprs-pre-imsi.sh PATH/TO/PATCHED/openbts}
test_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
patch_file=$(CDPATH= cd -- "$test_dir/../patches" && pwd)/0007-gprs-pre-imsi-ccch-assignment.patch
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM
extract() {
    awk -v pattern="$2" '
      $0 ~ pattern { copying=1; found++ }
      copying { print }
      copying && /^};?$/ { copying=0 }
      END { if (found != 1 || copying) exit 1 }
    ' "$1" > "$3"
}
# This fix is specific to the pinned public-release FIFO pager. Do not silently
# carry it to a paging-group implementation which needs IMSI-derived DRX slots.
if grep -E 'getImsiMod1000|mImsi' "$source_root/GSM/GSMCCCH.cpp" >/dev/null; then
    echo 'FAIL: IMSI-dependent pager requires a new pre-IMSI delivery review' >&2
    exit 1
fi
grep -F 'void addPage(NewPagingEntry *npe) { mPageQ.write(npe); }' "$source_root/GSM/GSMCCCH.cpp" >/dev/null
grep -F 'gPagingQ.addPage(gprsMsg);' "$source_root/GSM/GSMCCCH.cpp" >/dev/null
compile_case() {
    root=$1
    out=$2
    mkdir -p "$out"
    extract "$root/Control/PagingEntry.h" '^struct NewPagingEntry [{]' "$out/paging-entry.inc"
    extract "$root/Control/PagingEntry.cpp" '^NewPagingEntry::~NewPagingEntry[(]' "$out/paging-destructor.inc"
    extract "$root/GPRS/TBF.cpp" '^uint32_t TBF::mtGetTlli[(]' "$out/tlli-method.inc"
    extract "$root/GPRS/TBF.cpp" '^L3ImmediateAssignment [*]gprsPageCcchStart[(]' "$out/assignment-start.inc"
    extract "$root/GPRS/TBF.cpp" '^void sendAssignmentCcch[(]' "$out/assignment-send.inc"
    extract "$root/GSM/GSMCCCH.cpp" '^bool CCCHLogicalChannel::processPages[(]' "$out/process-pages.inc"
    ${CXX:-c++} -std=gnu++11 -Wall -Wextra -Werror -Wno-unused-parameter -I "$out" \
        "$test_dir/gprs-pre-imsi.cpp" -o "$out/gprs-pre-imsi"
}
compile_case "$source_root" "$workspace/patched"
"$workspace/patched/gprs-pre-imsi"
mkdir -p "$workspace/original/GPRS" "$workspace/original/GSM" "$workspace/original/Control"
cp "$source_root/GPRS/TBF.cpp" "$workspace/original/GPRS/"
cp "$source_root/GSM/GSMCCCH.cpp" "$workspace/original/GSM/"
cp "$source_root/Control/PagingEntry.h" "$source_root/Control/PagingEntry.cpp" "$workspace/original/Control/"
(CDPATH= cd -- "$workspace/original"; patch -s -R -p1 < "$patch_file")
compile_case "$workspace/original" "$workspace/negative"
"$workspace/negative/gprs-pre-imsi" known-only
status=0
"$workspace/negative/gprs-pre-imsi" >"$workspace/negative.log" 2>&1 || status=$?
test "$status" -eq 17
grep -F 'EXPECTED-NEGATIVE: original drops pre-IMSI assignment' "$workspace/negative.log" >/dev/null
echo 'PASS: original negative control drops pre-IMSI assignment (exit 17)'
