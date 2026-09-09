#!/bin/sh
# Offline contract for tag identity, cloud image testing and immutable publication.
# tag 身份、云端镜像测试及不可变发布的离线契约。
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
WORKFLOW=.github/workflows/release.yml
fail() { echo "FAIL [RELEASE-WORKFLOW] $1" >&2; exit 1; }
require() { grep -F -- "$1" "$WORKFLOW" >/dev/null || fail "missing: $1"; }

[ -f "$WORKFLOW" ] || fail 'release workflow is missing'
require "- 'v*.*.*'"
require 'workflow_dispatch:'
require 'dry_run:'
require 'tag:'
require 'ref:'
require '[[ "$REQUESTED_TAG" == "$tag" ]]'
require '[[ -s "docs/releases/$version.md" ]]'
require 'git show-ref --verify --quiet "refs/tags/$tag"'
require 'git merge-base --is-ancestor "$revision" refs/remotes/origin/master'
require '[[ "$HUB_REPOSITORY" == addxemmm/gsm-system ]]'
require 'persist-credentials: false'

# Actions must use immutable 40-character commit SHAs, never floating tags.
if grep -Eq 'uses:[[:space:]]+[^ @]+@(v[0-9]+|main|master)([[:space:]#]|$)' "$WORKFLOW"; then
  fail 'a GitHub Action uses a floating ref'
fi
awk '
  /uses:[[:space:]]/ {
    ref=$0; sub(/^.*@/, "", ref); sub(/[[:space:]#].*$/, "", ref)
    if (ref !~ /^[0-9a-f]{40}$/) exit 1
    count++
  }
  END { if (count < 6) exit 1 }
' "$WORKFLOW" || fail 'Actions are not pinned to full commit SHAs'

require 'group: gsm-system-dockerhub-release'
require 'cancel-in-progress: false'
require 'contents: read'
require 'contents: write'
require 'if: needs.source.outputs.dry_run == '\''true'\'''
require 'if: needs.source.outputs.dry_run != '\''true'\'''
require 'DOCKERHUB_USERNAME: ${{ secrets.DOCKERHUB_USERNAME }}'
require 'DOCKERHUB_TOKEN: ${{ secrets.DOCKERHUB_TOKEN }}'
require 'sh scripts/ci-release.sh inspect "$HUB_REPOSITORY" "$VERSION"'
require 'sh scripts/ci-release.sh inspect "$HUB_REPOSITORY" "$LINE"'

# The official build action loads one full Dockerfile build; both remote tags
# then point at the inspected and tested image ID.
require 'uses: docker/build-push-action@10e90e3645eae34f1e60eeb005ba3a3d33f178e8'
require 'file: deploy/docker/Dockerfile'
require 'platforms: linux/amd64'
require 'load: true'
require 'push: false'
require 'tags: gsm-system:2.1'
require 'VERSION=${{ needs.source.outputs.version }}'
require 'REVISION=${{ needs.source.outputs.rev12 }}'
require 'provenance: false'
require 'RUNTIME_IMAGE: gsm-system:2.1'
require 'docker tag "$IMAGE_ID" "$version_ref"'
require 'docker push "$version_ref"'
require 'docker tag "$source_image" "$stable_ref"'
require 'docker push "$stable_ref"'
if grep -Eq 'docker[.]io/[^[:space:]`"]+:(latest|v[0-9]|[0-9]+[.][0-9]+[.][0-9]+-)' "$WORKFLOW"; then
  fail 'latest, v-prefixed, or revision-suffixed Docker tag found'
fi
if grep -Eq '(^|[[:space:]])(git tag|git push)([[:space:]]|$)' "$WORKFLOW"; then
  fail 'workflow creates or rewrites a Git ref'
fi

for gate in \
  'go vet ./...' \
  'go test ./...' \
  'go test -race ./...' \
  'node scripts/tests/web_ui.test.cjs' \
  'node scripts/tests/postman_start_modes.test.cjs' \
  'sh scripts/tests/test_publication_inputs.sh' \
  'sh scripts/tests/test_tag_release.sh'
do
  require "$gate"
done
require 'sh scripts/tests/test_callerid_image.sh gsm-system:2.1'
require 'sh scripts/tests/test_image.sh gsm-system:2.1'
require 'sh scripts/tests/test_sms_image.sh gsm-system:2.1'
require 'sh scripts/tests/test_presets_image.sh gsm-system:2.1'
require 'sh scripts/tests/test_web_image.sh gsm-system:2.1'

require 'sh scripts/prefetch_vendor.sh'
require 'sh scripts/release/source_bundle.sh "$assets"'
require 'cp docs/THIRD-PARTY.md "$assets/THIRD-PARTY.md"'
require 'IMAGE-DIGEST.txt'
require 'BUILD-CHECKPOINT.txt'
require 'PUBLISHED-METADATA.txt'
require 'PUBLISHED-SHA256SUMS.txt'
require 'SHA256SUMS.txt'
require 'gsm-system-corresponding-source.tar.gz.sha256'
require 'gh release create "$TAG" --verify-tag --draft --latest=false'
require "echo 'action=resume_finalize'"
require 'ensure_asset IMAGE-DIGEST.txt "$post/IMAGE-DIGEST.txt"'
require 'Required draft asset missing or duplicated:'
require 'gh release edit "$TAG" --draft=false --latest=false'
require 'Physical GPRS handset'
require '不使用射频、USB、宿主网络或真实业务数据'
require 'cat "docs/releases/$VERSION.md" >>"$RUNNER_TEMP/release-notes.md"'
draft_line=$(grep -nF 'gh release create "$TAG" --verify-tag --draft' "$WORKFLOW" | cut -d: -f1)
push_line=$(grep -nF 'docker push "$version_ref"' "$WORKFLOW" | cut -d: -f1)
marker_line=$(grep -nF 'ensure_asset IMAGE-DIGEST.txt "$post/IMAGE-DIGEST.txt"' "$WORKFLOW" | cut -d: -f1)
asset_gate_line=$(grep -nF 'Require every release artifact before promotion' "$WORKFLOW" | cut -d: -f1)
promotion_line=$(grep -nF 'Promote the recorded image to the stable line' "$WORKFLOW" | cut -d: -f1)
[ "$draft_line" -lt "$push_line" ] || fail 'draft checkpoint is not created before immutable push'
[ "$push_line" -lt "$marker_line" ] || fail 'digest completion marker is not written after immutable push'
[ "$asset_gate_line" -lt "$promotion_line" ] || fail 'release assets are not checked before stable promotion'

# Execute the exact preflight shell block with a draft checkpoint and a remote
# full-version manifest, but without IMAGE-DIGEST.txt. This is the interruption
# window after the immutable push and before its completion marker; it must
# resume finalization without rebuilding or overwriting the full tag.
# 用真实工作流 shell 段模拟“完整版本已推送、摘要附件尚未上传”的中断窗口。
TMP=${TMPDIR:-/tmp}/gsm-release-workflow-$$
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
mkdir -p "$TMP/bin" "$TMP/scripts" "$TMP/run"
awk '
  /- name: Authenticate read-only and establish release state/ { found=1 }
  found && /run: \|/ { capture=1; next }
  capture && /^  publish:/ { exit }
  capture { sub(/^          /, ""); print }
' "$WORKFLOW" >"$TMP/preflight.sh"
cat >"$TMP/scripts/ci-release.sh" <<'SH'
#!/bin/sh
case "$3" in
  2.1.0)
    printf '%s\n' '{"state":"present","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","config_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","version":"2.1.0","revision":"0123456789ab"}' ;;
  2.1) printf '%s\n' '{"state":"absent"}' ;;
  *) exit 2 ;;
esac
SH
cat >"$TMP/release.json.fixture" <<'JSON'
{"draft":true,"assets":[{"name":"BUILD-CHECKPOINT.txt"}]}
JSON
cat >"$TMP/checkpoint.fixture" <<'EOF'
version=2.1.0
revision=0123456789abcdef0123456789abcdef01234567
repository=addxemmm/gsm-system
platform=linux/amd64
config_digest=sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
EOF
cat >"$TMP/bin/gh" <<'SH'
#!/bin/sh
case "$1:$2" in
  api:*) cat "$MOCK_ROOT/release.json.fixture" ;;
  release:download)
    pattern=
    while [ "$#" -gt 0 ]; do
      if [ "$1" = --pattern ]; then pattern=$2; shift 2; continue; fi
      if [ "$1" = --dir ]; then destination=$2; shift 2; continue; fi
      shift
    done
    printf '%s\n' "$pattern" >>"$MOCK_ROOT/downloads.log"
    [ "$pattern" = BUILD-CHECKPOINT.txt ] || exit 91
    cp "$MOCK_ROOT/checkpoint.fixture" "$destination/BUILD-CHECKPOINT.txt" ;;
  *) exit 92 ;;
esac
SH
cat >"$TMP/bin/jq" <<'SH'
#!/bin/sh
if [ "$1" = -r ] && [ "$2" = .draft ]; then printf 'true\n'; exit; fi
if [ "$1" = -er ] && [ "$2" = --arg ] && [ "$3" = key ]; then
  key=$4
  body=$(cat)
  printf '%s\n' "$body" | sed -n "s/.*\"$key\":\"\([^\"]*\)\".*/\1/p"
  exit
fi
case "$1" in
  *IMAGE-DIGEST.txt*) printf '0\n' ;;
  *) exit 93 ;;
esac
SH
chmod +x "$TMP/bin/gh" "$TMP/bin/jq" "$TMP/scripts/ci-release.sh"
(
  cd "$TMP"
  PATH="$TMP/bin:$PATH" MOCK_ROOT=$TMP RUNNER_TEMP="$TMP/run" \
    GITHUB_OUTPUT="$TMP/output" GITHUB_REPOSITORY=addxemmm/gsm-system \
    TAG=v2.1.0 VERSION=2.1.0 LINE=2.1 \
    REVISION=0123456789abcdef0123456789abcdef01234567 REV12=0123456789ab \
    HUB_REPOSITORY=addxemmm/gsm-system DOCKERHUB_USERNAME=fixture DOCKERHUB_TOKEN=fixture \
    bash "$TMP/preflight.sh"
)
grep -Fx 'action=resume_finalize' "$TMP/output" >/dev/null || fail 'post-push/pre-marker interruption is not resumable'
if grep -Fx 'IMAGE-DIGEST.txt' "$TMP/downloads.log" >/dev/null; then
  fail 'resume preflight tried to require the not-yet-written completion marker'
fi

sh scripts/ci-release.sh self-test >/dev/null
echo 'PASS test_release_workflow.sh / 自动发布工作流契约通过'
