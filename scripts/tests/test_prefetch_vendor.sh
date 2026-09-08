#!/bin/sh
# Offline prefetch regression tests using tiny local Git fixtures.
# 使用本地微型 Git 仓库验证锁定、原子安装及 dirty 拒绝逻辑。
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
TMP=${TMPDIR:-/tmp}/gsm-prefetch-test-$$
SOURCES=$TMP/sources
CACHE=$TMP/cache
mkdir -p "$SOURCES"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT HUP INT TERM

make_repo() {
  name=$1
  shift
  repo=$SOURCES/$name
  git init -q -b fixture "$repo"
  for path in "$@"; do
    mkdir -p "$(dirname "$repo/$path")"
    printf 'fixture %s %s\n' "$name" "$path" >"$repo/$path"
  done
  git -C "$repo" add .
  git -c user.name=fixture -c user.email=fixture@example.invalid -C "$repo" \
    commit -qm "fixture $name"
}

make_repo uhd4 host/CMakeLists.txt
make_repo cppzmq zmq.hpp zmq_addon.hpp
make_repo openbts autogen.sh apps/OpenBTS.example.sql
make_repo smqueue autogen.sh
make_repo subscriberRegistry autogen.sh
make_repo liba53 Makefile
make_repo libcoredumper fix_from_scratch_build.patch

UHD_REV=$(git -C "$SOURCES/uhd4" rev-parse HEAD)
CPPZMQ_REV=$(git -C "$SOURCES/cppzmq" rev-parse HEAD)
OPENBTS_REVISION=$(git -C "$SOURCES/openbts" rev-parse HEAD)
SMQUEUE_REVISION=$(git -C "$SOURCES/smqueue" rev-parse HEAD)
SUBSCRIBER_REV=$(git -C "$SOURCES/subscriberRegistry" rev-parse HEAD)
LIBA53_REVISION=$(git -C "$SOURCES/liba53" rev-parse HEAD)
LIBCOREDUMPER_REVISION=$(git -C "$SOURCES/libcoredumper" rev-parse HEAD)

mkdir -p "$TMP/archive/coredumper-1.2.1"
printf 'archive fixture\n' >"$TMP/archive/coredumper-1.2.1/README"
tar -czf "$TMP/coredumper-1.2.1.tar.gz" -C "$TMP/archive" coredumper-1.2.1
ARCHIVE_SHA=$(sha256sum "$TMP/coredumper-1.2.1.tar.gz" | awk '{print $1}')
ARCHIVE_SIZE=$(wc -c <"$TMP/coredumper-1.2.1.tar.gz" | tr -d '[:space:]')

run_prefetch() {
  GSM_VENDOR_CACHE=${TEST_CACHE:-$CACHE} \
  UHD4_ORIGIN=$SOURCES/uhd4 UHD4_REV=${TEST_UHD_REV:-$UHD_REV} \
  CPPZMQ_ORIGIN=$SOURCES/cppzmq CPPZMQ_REV=$CPPZMQ_REV \
  OPENBTS_ORIGIN=$SOURCES/openbts OPENBTS_REV=$OPENBTS_REVISION \
  SMQUEUE_ORIGIN=$SOURCES/smqueue SMQUEUE_REV=$SMQUEUE_REVISION \
  SUBSCRIBER_REGISTRY_ORIGIN=$SOURCES/subscriberRegistry SUBSCRIBER_REGISTRY_REV=$SUBSCRIBER_REV \
  LIBA53_ORIGIN=$SOURCES/liba53 LIBA53_REV=$LIBA53_REVISION \
  LIBCOREDUMPER_ORIGIN=$SOURCES/libcoredumper LIBCOREDUMPER_REV=$LIBCOREDUMPER_REVISION \
  COREDUMPER_ARCHIVE_URL=file://$TMP/coredumper-1.2.1.tar.gz \
  COREDUMPER_ARCHIVE_SHA256=$ARCHIVE_SHA COREDUMPER_ARCHIVE_SIZE=$ARCHIVE_SIZE \
    "$ROOT/scripts/prefetch_vendor.sh"
}

run_prefetch >/dev/null
[ -f "$CACHE/REVISION_MANIFEST.tsv" ]
for pair in \
  uhd4:$UHD_REV cppzmq:$CPPZMQ_REV openbts:$OPENBTS_REVISION \
  smqueue:$SMQUEUE_REVISION subscriberRegistry:$SUBSCRIBER_REV \
  liba53:$LIBA53_REVISION libcoredumper:$LIBCOREDUMPER_REVISION
do
  component=${pair%%:*}
  revision=${pair#*:}
  awk -F '\t' -v component="$component" -v revision="$revision" \
    '$1 == component && $2 == "repository" && $4 == revision { found++ } END { exit found == 1 ? 0 : 1 }' \
    "$CACHE/REVISION_MANIFEST.tsv"
done
test "$(git -C "$CACHE/openbts" rev-parse HEAD)" = "$OPENBTS_REVISION"
echo 'PASS: exact revisions cloned and manifested / 精确 revision 已克隆并记录'

# Windows archive extraction can change executable bits; content remains strict.
chmod +x "$CACHE/openbts/autogen.sh"
run_prefetch >/dev/null
printf 'untracked\n' >"$CACHE/smqueue/generated-source.cpp"
if run_prefetch >/dev/null 2>&1; then
  echo 'untracked vendor content was accepted' >&2
  exit 1
fi
[ ! -e "$CACHE/REVISION_MANIFEST.tsv" ]
rm "$CACHE/smqueue/generated-source.cpp"
run_prefetch >/dev/null
printf 'dirty content\n' >>"$CACHE/openbts/autogen.sh"
if run_prefetch >/dev/null 2>&1; then
  echo 'dirty vendor content was accepted' >&2
  exit 1
fi
grep -F 'dirty content' "$CACHE/openbts/autogen.sh" >/dev/null
[ ! -e "$CACHE/REVISION_MANIFEST.tsv" ]
echo 'PASS: mode-only drift accepted; untracked/content drift rejected / 权限漂移可接受，未跟踪及内容改动拒绝'

# A fetch/lock failure must leave neither the destination nor a stale manifest.
BAD_CACHE=$TMP/bad-cache
if TEST_CACHE=$BAD_CACHE TEST_UHD_REV=0000000000000000000000000000000000000000 \
  run_prefetch >/dev/null 2>&1; then
  echo 'invalid revision unexpectedly prefetched' >&2
  exit 1
fi
[ ! -e "$BAD_CACHE/uhd4" ]
[ ! -e "$BAD_CACHE/REVISION_MANIFEST.tsv" ]
if find "$BAD_CACHE" -maxdepth 1 -name '.prefetch-*' | grep . >/dev/null; then
  echo 'failed prefetch left a staging directory' >&2
  exit 1
fi
echo 'PASS test_prefetch_vendor.sh / vendor 预取脚本测试通过'
