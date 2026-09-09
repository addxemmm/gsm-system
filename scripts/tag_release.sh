#!/bin/sh
# Plan by default. Only --push creates/pushes a release tag; never bump VERSION.
# 默认只展示计划，--push 才创建并推送 tag，不自动升版。
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
mode=${1:---plan}
case "$mode" in --plan|--push) ;; *) echo 'Usage: sh scripts/tag_release.sh [--plan|--push]' >&2; exit 2;; esac
remote=${GSM_GIT_REMOTE:-github}
case "$remote" in ''|-*|*[!a-zA-Z0-9._-]*) echo 'Invalid Git remote' >&2; exit 2;; esac
version=$(git show HEAD:VERSION | tr -d '\r\n')
printf '%s\n' "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || { echo 'Invalid committed VERSION' >&2; exit 1; }
tag=v$version
revision=$(git rev-parse HEAD)
printf 'Version / 版本: %s\nCommit / 提交: %s\nTag: %s\nRemote / 远端: %s\n' "$version" "$revision" "$tag" "$remote"
if [ "$mode" = --plan ]; then
  echo 'Plan only. Commit reviewed VERSION/changes to master, then use --push. / 仅计划，先提交并审核 master，再执行 --push。'
  exit 0
fi
[ -z "$(git status --porcelain)" ] || { echo 'Clean checkout required / 工作区须干净' >&2; exit 1; }
git fetch "$remote" master --tags
git merge-base --is-ancestor HEAD "$remote/master" || { echo 'Commit must be merged to master / 提交须已进入 master' >&2; exit 1; }
if git show-ref --verify --quiet "refs/tags/$tag"; then
  echo 'Tag already exists; dispatch the existing tag for retry, never move it. / tag 已存在，重试请运行既有 tag，禁止移动。' >&2
  exit 1
fi
git tag -a "$tag" -m "gsm-system $version / GSM 系统 $version"
git push "$remote" "refs/tags/$tag"
echo 'Tag pushed; follow the release workflow. / tag 已推送，请查看 release 工作流。'
