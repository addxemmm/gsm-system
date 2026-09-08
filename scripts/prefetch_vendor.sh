#!/bin/sh
# Fetch and verify native upstream sources on the SDR build host.
# 在 SDR 构建服务器上预取并完整校验上游原生源码。
#
# Existing trees are never updated implicitly. Every HEAD and recursive submodule
# must match the release lock; tracked content changes fail (file-mode-only drift
# is ignored for archives copied through Windows). Missing trees are fetched into
# a same-filesystem temporary directory and moved into place only after checks.
# 已有源码不会被隐式更新；HEAD/子模块必须匹配发布锁，内容改动立即失败（仅忽略经
# Windows 复制造成的权限位变化）。缺失源码在同盘临时目录校验后再原子移入。
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
CACHE=${GSM_VENDOR_CACHE:-$ROOT/third_party}
STAGE=$CACHE/.prefetch-$$
MANIFEST_TMP=$CACHE/.REVISION_MANIFEST.tsv.$$
MANIFEST=$CACHE/REVISION_MANIFEST.tsv
COREDUMPER_ARCHIVE_URL=${COREDUMPER_ARCHIVE_URL:-https://storage.googleapis.com/google-code-archive-downloads/v2/code.google.com/google-coredumper/coredumper-1.2.1.tar.gz}
COREDUMPER_ARCHIVE_SHA256=${COREDUMPER_ARCHIVE_SHA256:-c7dc2f0ee2b00a3192bd1fa595d175ce6ecf6c5499220ba1232fa2fb4cc3774d}
COREDUMPER_ARCHIVE_SIZE=${COREDUMPER_ARCHIVE_SIZE:-437668}

UHD4_ORIGIN=${UHD4_ORIGIN:-https://github.com/EttusResearch/uhd.git}
UHD4_REV=${UHD4_REV:-d21735d543d5a3c265507965c7bb6c9e9df95fcd}
CPPZMQ_ORIGIN=${CPPZMQ_ORIGIN:-https://github.com/zeromq/cppzmq.git}
CPPZMQ_REV=${CPPZMQ_REV:-76bf169fd67b8e99c1b0e6490029d9cd5ef97666}
OPENBTS_ORIGIN=${OPENBTS_ORIGIN:-https://github.com/RangeNetworks/openbts.git}
OPENBTS_REV=${OPENBTS_REV:-7766ef94f2d885c197430e74a89f02740f0c04e6}
SMQUEUE_ORIGIN=${SMQUEUE_ORIGIN:-https://github.com/RangeNetworks/smqueue.git}
SMQUEUE_REV=${SMQUEUE_REV:-e168a262db311231c51cf7295f9bd1f440567485}
SUBSCRIBER_REGISTRY_ORIGIN=${SUBSCRIBER_REGISTRY_ORIGIN:-https://github.com/RangeNetworks/subscriberRegistry.git}
SUBSCRIBER_REGISTRY_REV=${SUBSCRIBER_REGISTRY_REV:-c65b5d59f744a8df5f3395e217f2598f53e9fe65}
LIBA53_ORIGIN=${LIBA53_ORIGIN:-https://github.com/RangeNetworks/liba53.git}
LIBA53_REV=${LIBA53_REV:-27354560dc7b554e03d40a520d41290e731193b6}
LIBCOREDUMPER_ORIGIN=${LIBCOREDUMPER_ORIGIN:-https://github.com/RangeNetworks/libcoredumper.git}
LIBCOREDUMPER_REV=${LIBCOREDUMPER_REV:-7527fb3804927c7fdc72ff5139a2cdea3db4d59a}

cleanup() {
  rc=$?
  trap - EXIT HUP INT TERM
  rm -rf "$STAGE"
  rm -f "$MANIFEST_TMP"
  # A failed verification invalidates any older manifest so Docker cannot
  # consume stale provenance for a changed cache.
  [ "$rc" -eq 0 ] || rm -f "$MANIFEST"
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

command -v git >/dev/null 2>&1 || { echo 'git is required / 需要 git' >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo 'sha256sum is required / 需要 sha256sum' >&2; exit 1; }
mkdir -p "$CACHE" "$STAGE"
printf 'component\tkind\trequested_ref\trevision_or_sha256\torigin\n' >"$MANIFEST_TMP"

record_submodules() {
  component=$1
  destination=$2
  status_file=$STAGE/submodules-$component
  git -C "$destination" submodule status --recursive >"$status_file"
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    marker=$(printf '%s' "$line" | cut -c1)
    case "$marker" in
      ' ') ;;
      -) echo "Uninitialized submodule in $component: $line" >&2; return 1 ;;
      +) echo "Submodule revision mismatch in $component: $line" >&2; return 1 ;;
      U) echo "Conflicted submodule in $component: $line" >&2; return 1 ;;
      *) echo "Invalid submodule status in $component: $line" >&2; return 1 ;;
    esac
    remainder=${line#?}
    revision=${remainder%% *}
    remainder=${remainder#* }
    path=${remainder%% *}
    git -C "$destination/$path" fsck --full --strict >/dev/null
    git -c core.fileMode=false -C "$destination/$path" diff --quiet
    git -c core.fileMode=false -C "$destination/$path" diff --cached --quiet
    submodule_untracked=$(git -C "$destination/$path" ls-files --others)
    [ -z "$submodule_untracked" ] || {
      echo "Untracked files in submodule $component/$path:" >&2
      printf '%s\n' "$submodule_untracked" >&2
      return 1
    }
    origin=$(git -C "$destination/$path" remote get-url origin 2>/dev/null || printf local-submodule)
    printf '%s/%s\trepository\trecorded-by-superproject\t%s\t%s\n' \
      "$component" "$path" "$revision" "$origin" >>"$MANIFEST_TMP"
  done <"$status_file"
}

verify_repository() {
  component=$1
  origin=$2
  requested_ref=$3
  destination=$4
  shift 4
  [ -d "$destination/.git" ] || {
    echo "Incomplete source tree (missing .git): $destination" >&2
    return 1
  }
  actual_origin=$(git -C "$destination" remote get-url origin)
  [ "$actual_origin" = "$origin" ] || {
    echo "Unexpected origin for $component: $actual_origin" >&2
    return 1
  }
  git -C "$destination" fsck --full --strict >/dev/null
  git -c core.fileMode=false -C "$destination" diff --quiet
  git -c core.fileMode=false -C "$destination" diff --cached --quiet
  untracked=$(git -C "$destination" ls-files --others)
  case "$component:$untracked" in
    libcoredumper:|libcoredumper:coredumper-1.2.1.tar.gz) ;;
    *:) ;;
    *)
      echo "Untracked files in $component:" >&2
      printf '%s\n' "$untracked" >&2
      return 1
      ;;
  esac
  revision=$(git -C "$destination" rev-parse --verify HEAD)
  case "$revision" in
    [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) ;;
    *) echo "Invalid revision for $component: $revision" >&2; return 1 ;;
  esac
  [ "$revision" = "$requested_ref" ] || {
    echo "$component HEAD $revision does not match pinned revision $requested_ref" >&2
    return 1
  }
  for required in "$@"; do
    [ -e "$destination/$required" ] || {
      echo "Missing required source: $component/$required" >&2
      return 1
    }
  done
  printf '%s\trepository\t%s\t%s\t%s\n' \
    "$component" "$requested_ref" "$revision" "$origin" >>"$MANIFEST_TMP"
  record_submodules "$component" "$destination"
}

ensure_repository() {
  component=$1
  origin=$2
  requested_ref=$3
  shift 3
  case "$requested_ref" in *[!0-9a-f]*) echo "invalid pinned revision: $requested_ref" >&2; return 1 ;; esac
  [ "${#requested_ref}" -eq 40 ] || { echo "pinned revision must contain 40 hex characters: $requested_ref" >&2; return 1; }
  destination=$CACHE/$component
  if [ ! -e "$destination" ]; then
    candidate=$STAGE/$component
    git init -q -b gsm-release "$candidate"
    git -C "$candidate" remote add origin "$origin"
    git -C "$candidate" fetch --depth 1 origin "$requested_ref"
    git -C "$candidate" checkout --detach FETCH_HEAD
    git -C "$candidate" submodule update --init --recursive --depth 1
    verify_repository "$component" "$origin" "$requested_ref" "$candidate" "$@"
    # The final move is atomic because STAGE is a sibling within third_party.
    # Recompute paths because POSIX sh function variables are global.
    mv "$STAGE/$component" "$CACHE/$component"
  else
    verify_repository "$component" "$origin" "$requested_ref" "$destination" "$@"
  fi
}

# Exact revisions verified from the deployed UHD4 source set on vm-sdr.
# 精确 revision 已与 vm-sdr 当前 UHD4 源码集核对。
ensure_repository uhd4 "$UHD4_ORIGIN" "$UHD4_REV" host/CMakeLists.txt
ensure_repository cppzmq "$CPPZMQ_ORIGIN" "$CPPZMQ_REV" zmq.hpp zmq_addon.hpp
ensure_repository openbts "$OPENBTS_ORIGIN" "$OPENBTS_REV" autogen.sh apps/OpenBTS.example.sql
ensure_repository smqueue "$SMQUEUE_ORIGIN" "$SMQUEUE_REV" autogen.sh
ensure_repository subscriberRegistry "$SUBSCRIBER_REGISTRY_ORIGIN" "$SUBSCRIBER_REGISTRY_REV" autogen.sh
ensure_repository liba53 "$LIBA53_ORIGIN" "$LIBA53_REV" Makefile
ensure_repository libcoredumper "$LIBCOREDUMPER_ORIGIN" "$LIBCOREDUMPER_REV" fix_from_scratch_build.patch

# libcoredumper's wrapper repository does not contain the archived 1.2.1 source.
# Fetch it outside Docker, validate archive structure, record its actual digest,
# and atomically install it. A failed/offline download never creates a fake rev.
verify_archive() {
  archive_path=$1
  gzip -t "$archive_path"
  archive_sha=$(sha256sum "$archive_path" | awk '{print $1}')
  [ "$archive_sha" = "$COREDUMPER_ARCHIVE_SHA256" ] || {
    echo "libcoredumper archive SHA-256 mismatch: $archive_sha" >&2
    return 1
  }
  archive_size=$(wc -c <"$archive_path" | tr -d '[:space:]')
  [ "$archive_size" = "$COREDUMPER_ARCHIVE_SIZE" ] || {
    echo "libcoredumper archive size mismatch: $archive_size" >&2
    return 1
  }
}

archive=$CACHE/libcoredumper/coredumper-1.2.1.tar.gz
if [ ! -f "$archive" ]; then
  command -v curl >/dev/null 2>&1 || { echo 'curl is required / 需要 curl' >&2; exit 1; }
  candidate=$STAGE/coredumper-1.2.1.tar.gz
  curl --fail --location --retry 3 --silent --show-error --output "$candidate" "$COREDUMPER_ARCHIVE_URL"
  gzip -t "$candidate"
  tar -tzf "$candidate" >"$STAGE/coredumper.contents"
  grep -q '^coredumper-1\.2\.1/' "$STAGE/coredumper.contents"
  if grep -Eq '(^/|(^|/)\.\.(/|$))' "$STAGE/coredumper.contents"; then
    echo 'Unsafe path in libcoredumper archive' >&2
    exit 1
  fi
  verify_archive "$candidate"
  mv "$candidate" "$archive"
fi
verify_archive "$archive"
archive_sha=$(sha256sum "$archive" | awk '{print $1}')
printf 'libcoredumper-1.2.1\tarchive\tupstream-archive\t%s\t%s\n' \
  "$archive_sha" "$COREDUMPER_ARCHIVE_URL" >>"$MANIFEST_TMP"

mv -f "$MANIFEST_TMP" "$MANIFEST"
trap - EXIT HUP INT TERM
rm -rf "$STAGE"
printf 'Verified source manifest / 已验证源码清单: %s\n' "$MANIFEST"
du -sh "$CACHE"
