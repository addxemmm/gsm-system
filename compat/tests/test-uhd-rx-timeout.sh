#!/bin/sh
# Compile production UHD receive methods and ring buffer with deterministic stubs.
# 编译提取的生产收包方法；仅模拟元数据与样本，不启动服务、时钟或射频。
set -eu

source_file=${1:?usage: test-uhd-rx-timeout.sh PATH/Transceiver52M/UHDDevice.cpp}
test_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
patch_file=$(CDPATH= cd -- "$test_dir/../patches" && pwd)/0005-uhd-rx-timeout-retry.patch
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM

extract_source() {
    input=$1
    output_dir=$2
    mkdir -p "$output_dir"

    awk '
      /^class smpl_buf / { copying=1; found++ }
      copying { print }
      copying && /^};/ { copying=0 }
      END { if (found != 1 || copying) exit 1 }
    ' "$input" > "$output_dir/smpl-buf-class.inc"

    awk '
      /^int uhd_device::(check_rx_md_err|readSamples)\(/ {
        copying=1; opened=0; depth=0; found++
      }
      copying {
        print
        line=$0
        opens=gsub(/\{/, "{", line)
        closes=gsub(/\}/, "}", line)
        if (opens) opened=1
        depth += opens - closes
        if (opened && depth == 0) copying=0
      }
      END { if (found != 2 || copying) exit 1 }
    ' "$input" > "$output_dir/uhd-rx-methods.inc"

    awk '
      /^smpl_buf::smpl_buf\(/ { copying=1; found++ }
      /^RadioDevice \*RadioDevice::make\(/ { copying=0 }
      copying { print }
      END { if (found != 1 || copying) exit 1 }
    ' "$input" > "$output_dir/smpl-buf-methods.inc"
}

compile_case() {
    source=$1
    expected=$2
    name=$3
    build_dir="$workspace/$name"
    extract_source "$source" "$build_dir"
    ${CXX:-c++} -std=gnu++14 -Wall -Wextra -Werror \
        -Wno-unused-parameter -Wno-vla -Wno-implicit-fallthrough -pthread \
        -DEXPECT_PATCHED="$expected" -I "$build_dir" \
        "$test_dir/uhd-rx-timeout.cpp" -o "$build_dir/uhd-rx-timeout"
}

grep -F 'ERROR_TIMEOUT = -4' "$source_file" >/dev/null
grep -F 'std::chrono::steady_clock' "$source_file" >/dev/null
compile_case "$source_file" 1 patched
"$workspace/patched/uhd-rx-timeout"

# Reconstruct the exact pre-patch source, then prove the recovery test fails there.
original_root="$workspace/original-tree"
mkdir -p "$original_root/Transceiver52M"
cp "$source_file" "$original_root/Transceiver52M/UHDDevice.cpp"
(
    CDPATH= cd -- "$original_root"
    patch -s -R -p1 < "$patch_file"
)
compile_case "$original_root/Transceiver52M/UHDDevice.cpp" 0 original
if "$workspace/original/uhd-rx-timeout" >"$workspace/original.log" 2>&1; then
    cat "$workspace/original.log" >&2
    echo 'FAIL: original negative control unexpectedly recovered' >&2
    exit 1
fi
grep -F 'EXPECTED-NEGATIVE: original exits on first transient timeout' \
    "$workspace/original.log" >/dev/null
echo 'PASS: original negative control reproduces immediate timeout exit'
