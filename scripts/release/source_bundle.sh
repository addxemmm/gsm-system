#!/bin/sh
# Package the exact prefetched source and local build modifications, not live data.
# 打包已预取的源码和本地构建修改，不包含业务数据。
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
OUTPUT=${1:?usage: source_bundle.sh OUTPUT_DIR}
mkdir -p "$OUTPUT"
OUTPUT=$(CDPATH= cd -- "$OUTPUT" && pwd)
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT HUP INT TERM
cd "$ROOT"
test -f third_party/REVISION_MANIFEST.tsv
sh scripts/prefetch_vendor.sh
go run ./scripts/release/publication --output-seeds "$STAGE/configs/seeds"
cp configs/app.yaml.example "$STAGE/configs/"
cp configs/seeds/OpenBTSDo "$STAGE/configs/seeds/"
# Whitelist build inputs; do not package archived databases, logs or environment files.
# 白名单只收录构建输入；不打包归档数据库、日志或环境文件。
tar --exclude=.git --exclude=__pycache__ --exclude='*.pyc' -cf - \
  third_party compat cmd internal deploy/docker scripts/release scripts/prefetch_vendor.sh \
  firmware gsmsystem/asterisk go.mod go.sum VERSION LICENSE .dockerignore README.md docs/THIRD-PARTY.md \
  > "$STAGE/inputs.tar"
tar -xf "$STAGE/inputs.tar" -C "$STAGE"
rm "$STAGE/inputs.tar"
# Include selected Go dependency sources and their original notices as well.
# 同时收录 Go 构建依赖源码和原始许可；不修改项目工作树。
go mod vendor -o "$STAGE/vendor"
cp third_party/REVISION_MANIFEST.tsv "$STAGE/VENDOR-REVISIONS.tsv"
git rev-parse HEAD > "$STAGE/SOURCE-REVISION"
cat > "$STAGE/BUILD-FROM-SOURCE.md" <<'EOF'
# Build the corresponding source / 构建对应源码

This archive already includes the pinned native sources and clean seeds. Do not
run prefetch_vendor.sh here: upstream Git metadata was intentionally excluded.
源码包已含固定原生源码和干净种子，不要在此运行预取脚本：Git 元数据已明确排除。

On a Linux Docker build host (network access for base images, apt and Go modules):
在 Linux Docker 构建主机上执行（拉取基础镜像、apt 与 Go 依赖需要网络）：

```sh
docker build -f deploy/docker/Dockerfile \
  --build-arg VERSION="$(cat VERSION)" \
  --build-arg REVISION="$(cut -c1-12 SOURCE-REVISION)" \
  -t gsm-system:2.1 .
```

This builds an image only, without starting RF or containers. The source revision
and archive checksum identify inputs, not a promise of bit-for-bit reproducibility
of mutable distribution packages. Component terms are in docs/THIRD-PARTY.md and
their original source trees. Never add live data or credentials to this archive.
以上仅构建，不启动射频或容器。源码 revision/归档摘要用于追踪输入，不承诺可变发行版
系统包的逐位可复现性。组件条款见第三方文档及原始源码，不向归档添加现网数据或密钥。
EOF
tar --sort=name --mtime="@${SOURCE_DATE_EPOCH:-0}" --owner=0 --group=0 --numeric-owner \
  -czf "$OUTPUT/gsm-system-corresponding-source.tar.gz" -C "$STAGE" .
(cd "$OUTPUT" && sha256sum gsm-system-corresponding-source.tar.gz > gsm-system-corresponding-source.tar.gz.sha256)
printf 'Source archive ready / 对应源码包已生成\n'
