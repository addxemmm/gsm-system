# Release workflow / 发布、版本记录与镜像同步

## Current boundary / 当前边界

- Source remains **2.1.0**, production image remains **gsm-system:2.1**. Nothing here
  changes deployment, creates a tag, changes repository visibility, or uploads on commit.
  源码仍为 **2.1.0**，运行镜像仍为 **gsm-system:2.1**；本流程不改部署、不自动建 tag、
  不改变仓库可见性，也不在提交时上传。
- Docker Hub publication is feasible **after** the operator supplies a namespace,
  creates/reviews a **private** repository, enables immutable tags, and supplies credentials.
  DockerHub 上传技术上可行；先由操作者确定 namespace、创建并审核**私有仓库**、启用
  immutable tags、配置凭据。当前没有已上传镜像或成功发布的承诺。
- The Dockerfile requires untracked `third_party/` sources and a pinned manifest.
  `scripts/prefetch_vendor.sh` verifies pinned upstream commits/submodules and the archive
  checksum on the server. A GitHub checkout alone is **not** a build context.
  Dockerfile 依赖未入 Git 的服务器 `third_party/` 和固定供应链清单；云端 checkout
  不具备完整构建上下文。本流程只推送服务器已验收的最终镜像，不在云端重新构建。
- No self-hosted runner, PR trigger, automatic `latest`, image tar upload, container
  `commit/export`, volume export, or repository visibility change is configured.
  不配置自管 runner、不监听 PR、不推 latest、不上传镜像 tar、不导出容器或数据卷。

## Identity contract / 版本与追踪契约

| Identity / 标识 | Rule / 规则 |
| --- | --- |
| Source / 源码 | `VERSION` must match explicit input, currently `2.1.0` / 必须匹配输入 |
| GitHub release/tag | `v2.1.0`, **first creation only**, tag must already point to the exact accepted commit / 仅首次创建、既有 tag 对应验收提交 |
| Docker Hub artifact tag | `v2.1.0-<12-char revision>` / 同版本迭代使用内部 revision tag |
| Durable artifact / 持久产物 | `docker.io/NAMESPACE/REPOSITORY@sha256:...` / 记录 manifest digest |
| Runtime / 运行 | exactly `gsm-system:2.1`; no automatic retag or redeploy / 保持不变 |

The content digest binds the manifest and config; the verifier checks linux/amd64 and
OCI version/revision. Existing builds encode only 12 revision characters; this is
operator-linked provenance, **not** a signed build attestation or reproducible-build proof.
内容摘要校验 manifest/config、linux/amd64 和 OCI 元数据。现有构建只编码 12 位 revision；
完整 SHA 由操作者输入、工作流与 Git tag 核对，属于操作链追踪，不冒充签名构建证明。

For another accepted 2.1.0 iteration, publish a new internal revision image tag and add
a dated bilingual entry to `docs/RELEASE-2.1.md` before committing. Keep the previous
`v2.1.0` release and its digest unchanged. A new GitHub version requires the operator's
explicit version-change instruction, coordinated `VERSION`/build-contract/docs updates,
and fresh validation; the workflow never invents suffix versions or rewrites releases.
同一 2.1.0 迭代可上传新的内部 revision 镜像 tag，并在提交前补充带日期的双语发布记录；
原 `v2.1.0` Release/digest 永久保留。新 GitHub 版本需用户明确升版后同步修改版本、
构建契约和文档并重新验收；流程不自动造版本、不覆盖旧 Release。

## One-time preparation / 一次性准备

1. Verify GitHub visibility and review the staged source for public disclosure;
   never assume a source repository is private. On Docker Hub explicitly create the private destination and
   enable immutable tags (all tags, or a regex covering every version/revision tag).
   The preflight existence check is not a race lock; registry immutability is required.
   核验 GitHub 可见性并审查待提交源码，不假定源码仓库私有；显式创建私有 Hub 仓库并启用 immutable tags。脚本的存在性检查
   不是并发锁，真正防覆盖依赖 registry 不可变规则。
   [Docker immutable tags](https://docs.docker.com/docker-hub/repos/manage/hub-images/immutable-tags/).
2. Review **all final-image layers**, included seed databases, Asterisk configs and
   firmware redistribution rights before upload. OpenBTS, UHD, Asterisk and bundled firmware
   have their own licenses/source-offer/redistribution obligations; the project's MIT license
   does not replace or automatically satisfy them. Inspect locally on the server without
   saving live subscriber/Ki/IMSI, tokens, logs or data volumes into artifacts. Private
   repository status does not replace this review. The tool requires an explicit review
   acknowledgement but does not certify that an image is secret-free or query Hub visibility.
   上传前审核最终镜像所有层、种子数据库、Asterisk 配置及固件分发权；OpenBTS、UHD、
   Asterisk 和自带固件各自的许可证/源码提供/再分发义务不被项目 MIT 自动覆盖；私有不替代审查。
   不把现网签约数据、Ki/IMSI、令牌、日志、数据卷打入产物。审核参数为人工确认，
   脚本不冒充内容无密检测或 Hub 私有性 API 检查。
   Primary license references / 上游许可资料：
   [OpenBTS](https://github.com/RangeNetworks/openbts),
   [UHD licensing](https://kb.ettus.com/Licensing_FAQ),
   [Asterisk licensing](https://docs.asterisk.org/About-the-Project/License-Information/).
   The bundled clone-board firmware provenance file identifies a vendor bundle, not a
   redistribution grant; obtain the vendor's terms before external publication.
   自带克隆板固件的 provenance 文件仅说明 vendor bundle 来源，不是再分发许可；
   外部分发前补齐供应商条款。
3. Configure GitHub environment **release**: required reviewer(s), default-branch-only
   deployment rule, environment secrets `DOCKERHUB_USER` and **read-only**
   `DOCKERHUB_READ_TOKEN`. Protect the default branch and release tags from force-update
   and deletion. Environment protection is an operator setup step, not created by YAML.
   配置 release environment 的审核人、仅默认分支规则及只读 Hub 凭据；保护默认分支和
   tag 禁止强推/删除。YAML 不会代替操作者创建审核保护。不要让 PR 取得发布密钥。
4. The server needs Docker CLI, Go (compatible with go.mod), HTTPS access to Docker Hub,
   and a Hub credential with write access to this repository. Use a credential store or
   stdin login, never PAT in arguments, Git, YAML or logs. Workflow uses a separate read PAT.
   服务器需 Docker CLI、Go、Hub HTTPS 网络和该仓库写权限；凭据走 credential store/stdin，
   云端使用独立只读 PAT。
   [Docker login](https://docs.docker.com/reference/cli/docker/login/).

## Publish an accepted server image / 上传已验收服务器镜像

Run from the repository root **on the SDR server**, after the normal Windows sync and
`scripts/deploy_to_ubuntu.sh` gates, source CI, and documented hardware/SMS/voice/GPRS
acceptance. The sync writes `.release-revision`; an archive deployment need not contain Git.
只在 SDR 服务器项目根执行，前提是正常同步/部署门禁、源码 CI 与实机验收已通过。
同步生成 `.release-revision`，服务器归档目录不要求有 Git。

```sh
# Replace values; REVISION is the full accepted source SHA / 替换实际值，使用完整 SHA。
VERSION=2.1.0
REVISION=FULL_40_CHARACTER_ACCEPTED_COMMIT
HUB_REPOSITORY=NAMESPACE/REPOSITORY

# Offline plan; also allowed on the development machine / 离线计划，开发机可执行。
go run ./scripts/release --version "$VERSION" --revision "$REVISION" \
  --repository "$HUB_REPOSITORY"

# Obtain secrets interactively or via the server secret manager, not shell history.
# 从交互输入/服务器密钥管理器导出 DOCKERHUB_USER、DOCKERHUB_TOKEN，不把字面 PAT 写入命令。
printf '%s' "$DOCKERHUB_TOKEN" | docker login --username "$DOCKERHUB_USER" --password-stdin

# Explicit upload. No build/start/restart or volume operation / 显式上传，不构建/重启/操作卷。
go run ./scripts/release --mode publish --version "$VERSION" --revision "$REVISION" \
  --repository "$HUB_REPOSITORY" --acceptance-passed --private-repo-reviewed \
  --immutable-tags-confirmed
unset DOCKERHUB_TOKEN
```

The command checks deployment marker, current managed container health, local image ID,
OCI labels and running binary version. It tags the **image ID** (not a mutable local alias)
and pushes exactly one new remote tag. It then hashes the downloaded remote manifest/config
and verifies the remote config digest equals the accepted local image ID. Record the emitted
JSON digest in the bilingual release entry or internal acceptance record; never paste secrets.
命令核对部署标记、容器健康、镜像 ID、OCI 与运行二进制版本，以镜像 ID 推送单一新 tag，
再核验远端 manifest/config 与本地 ID 一致。将输出 digest 保存到双语迭代/内部验收记录。

If push succeeds but later verification fails, the upload may already exist. Investigate by
digest and use `--mode verify`; never remove or overwrite it just to rerun. A race is stopped
by Hub immutable tags. Only a registry HTTP 404 counts as absence; auth/network failures stop.
若推送成功而后置校验失败，远端可能已经存在；按 digest 调查并 verify，不删改 tag 以重跑。
只有明确 404 视为不存在，认证/限流/网络错误均停止。工具不自动清理任何 GSM/LTE 镜像。

## Record the first GitHub release / 首次 GitHub Release

The pipeline must be committed to the default branch and present in the accepted commit.
Create and push `vVERSION` manually **only after explicit operator approval**; it must point
to that same commit. The workflow itself never creates/moves tags, and `gh release create`
uses `--verify-tag`. No tag or release is created by following the plan command above.
流程文件需已提交至默认分支并包含在验收提交中。仅经用户明确批准后手工建立/推送对应
tag；workflow 不创建/移动 tag，Release 强制 `--verify-tag`。
[GitHub release CLI](https://cli.github.com/manual/gh_release_create).

In Actions select **release / 手动发布记录** on the default branch. Supply source version,
full revision, private Hub repository, pushed manifest digest, and acceptance confirmation.
Leave `create_release=false` for a verification-only run. Review the bilingual job summary;
then explicitly dispatch with `create_release=true` for the first release. The workflow runs
Go tests/vet and offline deployment contracts, verifies the remote image, and creates one
release without assets or `latest` promotion. It fails if that release already exists.
在默认分支手动触发，填写版本、完整 SHA、Hub 仓库、digest 和验收确认；默认只校验。
审核双语 summary 后，显式选择 create_release 才首次建 Release。旧 Release 已存在则停止。
不上传源码/日志/镜像 tar 附件，不标记 latest，不自动拉取或部署到服务器。
[Manual workflow dispatch](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow).

This is **explicit promotion with automated checks and record synchronization**, not an
unattended deployment loop. For rollback retain the prior digest/version/revision and use
the existing deployment gates and matching source checkout; do not bypass them with Compose.
这是**人工批准后自动校验/同步记录**，不是无人值守部署。回滚保留上一 digest/版本/revision，
配合匹配源码走既有部署门禁；不要直接 Compose 启动。

## Offline validation / 离线测试

```sh
go test ./scripts/release
go vet ./scripts/release
```

Tests cover strict input handling, side-effect-free plan, manifest/config tampering, OCI
mismatch and untested multi-platform rejection. These tests do not demonstrate real Hub
credentials, repository configuration, registry reachability, Actions approval or RF success.
离线测试覆盖严格输入、无副作用计划、摘要篡改、OCI 错配和多平台拒收；不代表已验证
真实 Hub 凭据/配置/连通性、Actions 审批或射频效果。
