#!/bin/sh
# Offline tag-helper tests using fake Git; never contact or modify a repository.
# 假 Git 离线测试，不连接远端或修改仓库。
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
export CALLS="$TMP/calls"
mkdir "$TMP/bin"
cat > "$TMP/bin/git" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$CALLS"
case "$1" in
  show) printf '%s\n' "${TEST_VERSION:-2.1.0}";;
  rev-parse) printf '%040d\n' 1;;
  status) printf '%s' "${TEST_DIRTY:-}";;
  merge-base) exit "${TEST_ANCESTRY:-0}";;
  show-ref) exit "${TEST_EXISTS:-1}";;
  fetch|tag|push) exit 0;;
  *) exit 9;;
esac
EOF
chmod +x "$TMP/bin/git"
export PATH="$TMP/bin:$PATH"
reset_calls() { : > "$CALLS"; }
no_push() { ! grep -E '^(tag|push) ' "$CALLS" >/dev/null; }
reset_calls
sh "$ROOT/scripts/tag_release.sh" > "$TMP/plan"
grep -F '2.1.0' "$TMP/plan" >/dev/null
! grep -E '^(fetch|tag|push) ' "$CALLS" >/dev/null
reset_calls
if TEST_VERSION=2.01.0 sh "$ROOT/scripts/tag_release.sh" > /dev/null 2>&1; then exit 1; fi
no_push
reset_calls
if TEST_DIRTY=' M file' sh "$ROOT/scripts/tag_release.sh" --push > /dev/null 2>&1; then exit 1; fi
no_push
reset_calls
if TEST_ANCESTRY=1 sh "$ROOT/scripts/tag_release.sh" --push > /dev/null 2>&1; then exit 1; fi
no_push
reset_calls
if TEST_EXISTS=0 sh "$ROOT/scripts/tag_release.sh" --push > /dev/null 2>&1; then exit 1; fi
no_push
reset_calls
sh "$ROOT/scripts/tag_release.sh" --push > /dev/null
grep -F 'fetch github master --tags' "$CALLS" >/dev/null
grep -F 'tag -a v2.1.0 -m ' "$CALLS" >/dev/null
grep -Fx 'push github refs/tags/v2.1.0' "$CALLS" >/dev/null
reset_calls
if GSM_GIT_REMOTE='--upload-pack=bad' sh "$ROOT/scripts/tag_release.sh" > /dev/null 2>&1; then exit 1; fi
no_push
echo 'PASS tag helper / tag 脚本离线测试通过'
