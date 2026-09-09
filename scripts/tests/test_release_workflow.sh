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
require 'BUILD-METADATA.txt'
require 'SHA256SUMS.txt'
require 'gh release create "$TAG" --verify-tag --draft --latest=false'
require 'gh release edit "$TAG" --draft=false --latest=false'
require 'Physical GPRS handset'
require '不使用射频、USB、宿主网络或真实业务数据'

sh scripts/ci-release.sh self-test >/dev/null
echo 'PASS test_release_workflow.sh / 自动发布工作流契约通过'
