# Releases and Docker Hub / 版本、自动发版与镜像同步

## Version policy / 版本规则

| Item / 项目 | Contract / 契约 |
| --- | --- |
| Release line / 发布线 | **2.1**, unchanged until explicitly requested / 用户明确要求后才升级 |
| Source version / 源码版本 | `VERSION`, currently **2.1.0**, reviewed changes only, never automatic / 审核后修改，不自动升版 |
| Git tag and GitHub Release | **v2.1.0**, bound to one exact commit / 对应唯一提交 |
| Fixed version image / 固定版本镜像 | **addxemmm/gsm-system:2.1.0**, never replace with a different artifact / 不以新产物覆盖 |
| Release-line alias / 发布线别名 | **addxemmm/gsm-system:2.1**, promoted after verification / 验证通过后更新 |
| Durable identity / 持久标识 | Manifest `sha256` digest + full Git commit / 镜像摘要与完整提交 |
| Existing server / 现有服务器 | **gsm-system:2.1**, explicit deployment only / 仅显式部署 |

No `latest`, revision-suffixed public tags, automatic version bumps or automatic
server restarts. Ordinary commits run CI, not a Docker Hub release. A future
explicitly approved patch such as `2.1.1` produces `v2.1.1`, image `2.1.1` and
updates alias `2.1`. Older full-version images and release records remain.

不发布 latest 或奇怪尾号，不自动升版或重启服务器。普通提交只跑 CI；以后用户明确
升级为例如 2.1.1 时，创建对应 tag、固定版本镜像并更新 2.1。服务器缓存清理不等于
删除注册中心历史版本。

## Pipeline / 自动化流程

```text
reviewed master commit / master 已审核提交
       │
       ├─ dry-run / 可选全流程预检（不上传）
       └─ push vX.Y.Z / 推送版本 tag
                ↓
       validate tag + VERSION + ancestry / 版本与源码校验
                ↓
       fetch pinned native sources / 核验固定上游源码
                ↓
       Go + Web + Postman + contracts / 源码与契约测试
                ↓
       build + five isolated image suites / 构建与隔离验收
                ↓
       push the tested image ID / 上传同一个已测试镜像
                ↓
       verify digest + promote 2.1 / 核验摘要并更新别名
                ↓
       bilingual Release + source bundle / 双语记录与源码包
```

`.github/workflows/release.yml` runs on ephemeral GitHub Linux runners. A fresh
checkout is completed by `scripts/prefetch_vendor.sh`, verifying pinned commits,
submodules and the coredumper archive checksum. Builder caching speeds later
runs. No live container, volume, log or operator `.env` is exported. No self-hosted
runner is installed on the SDR server and no USB/RF access is used.

工作流在临时 GitHub Linux runner 执行，先预取并核验完整原生依赖，不依赖服务器未提交
目录。缓存加速后续构建；不导出现网容器、数据卷、日志或站点配置，不安装射频服务器
自管 runner，不接 USB 或启动射频。

Five final-image suites cover caller ID, persistence/Asterisk/ODBC/timezone, SMS,
presets and Web. They do **not** prove handset RF, call quality, SMS latency or
GPRS Internet connectivity. Hardware acceptance stays separate in
[2.1 history](RELEASE-2.1.md).

五套镜像测试覆盖主叫号码、持久化/Asterisk/ODBC/时区、短信、预设和 Web；不替代手机
射频、通话质量、短信时延或 GPRS 真机验收。

## GitHub configuration / GitHub 配置

Repository **addxemmm/gsm-system** → Settings → Secrets and variables → Actions:

| Kind / 类型 | Name / 名称 | Value / 含义 |
| --- | --- | --- |
| Secret | `DOCKERHUB_USERNAME` | Docker Hub account / Hub 用户名 |
| Secret | `DOCKERHUB_TOKEN` | Repository read/write PAT / 该镜像仓库读写 PAT |
| Variable | `DOCKERHUB_IMAGE` | `addxemmm/gsm-system` |

Do not put token values in Git, command arguments, Postman or release notes.
PRs do not receive publication credentials. Visibility is not changed by YAML.
令牌值不进入 Git、命令参数、Postman 或发布记录。PR 不获得发布密钥，工作流不改仓库可见性。

For registry-enforced immutability, configure a Hub immutable-tag rule for full
versions only, e.g. `^[0-9]+\.[0-9]+\.[0-9]+$`; keep `2.1` mutable. The workflow
also serializes publication and checks existing artifacts. Protect `master` and
`v*` tags against unreviewed changes, force-push and deletion. These account
settings are separate from the workflow file.

建议 Hub 不可变规则仅匹配三段固定版本，不锁定 2.1 别名；工作流另有串行发布与既有
产物检查。对 master 和版本 tag 设置审核及禁止强推/删除；账号保护不由 YAML 自动创建。
[Docker immutable tags](https://docs.docker.com/docker-hub/repos/manage/hub-images/immutable-tags/).

## Day-to-day release / 后续发版

1. Update code, bilingual docs and dated history. Change `VERSION` **only when
   explicitly approved**. 完成功能、双语文档和日期记录；用户批准升版后才改 VERSION。
   Add matching bilingual `docs/releases/X.Y.Z.md`; the workflow appends it to
   the generated validation/digest record. 同步建立该版本双语说明，工作流将其附到自动摘要与验收记录。
2. Test, commit, push and merge to `master`. In Actions run release with
   `dry_run=true` for full cloud build/tests without uploading.
   测试提交并进入 master，可先 dry-run 完整云构建验收而不上传。
3. From a clean checkout of the accepted commit / 在已验收提交的干净工作区：

   ```sh
   sh scripts/tag_release.sh --plan
   sh scripts/tag_release.sh --push
   ```

   The helper reads **committed** VERSION, verifies master contains the commit,
   and creates/pushes an annotated tag. It never bumps versions or moves tags.
   `GSM_GIT_REMOTE` defaults to `github`.
   脚本读取已提交 VERSION，确认 master 已含该提交，再推注解 tag；不改版本或移动 tag。
4. Follow Actions → release. Success produces bilingual notes, digest and source
   artifacts. Deployment to the SDR server remains a separate explicit action.
   查看 Actions；成功后生成双语记录、摘要和源码附件，部署到 SDR 仍需单独显式操作。

The tag must be pushed with a user/app credential. A workflow's GITHUB_TOKEN tag
push does not trigger another push workflow; no hidden automatic version-bump
job is used here. tag 使用用户凭据推送，不依赖隐藏自动升版任务。
[GitHub triggers](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).

## Retry and recovery / 重试与故障处理

- Re-run a failed Actions run or dispatch for the **existing** version tag. Never
  delete/move a published tag just to rerun. 重跑失败任务或选择既有 tag，不删改已发版本。
- Authentication, rate-limit and network errors are not proof a version is absent.
  Existing content must match before retrying. 认证/限流/网络失败不视为版本不存在。
- Push success followed by release creation failure can leave an image without a
  Release. Resume the same version; do not overwrite it with a rebuilt artifact.
  上传后 Release 失败时续跑同一版本，不用重建产物覆盖旧版本。
- A draft checkpoint is created before the first image push. If uploading a large
  source attachment is interrupted, retain the existing draft/assets and supply
  the missing matching attachments before retrying. Required assets are checked
  before alias promotion and publication; an incomplete source bundle is not silently
  published. 首次镜像推送前先建草稿检查点；大源码附件上传中断时，保留既有草稿与附件，
  补齐同一版本缺失附件后重跑。更新别名和发布前检查必需附件，不静默发布不完整源码。
- Older-version retries must not move `2.1` backwards. Source changes after a
  publication need an explicitly approved new patch version. Fix a failed dry-run
  before creating an immutable version tag. 旧版重试不回退 2.1，已发布后改源码需显式新版本。

## Pull and deploy / 拉取与部署

```sh
docker pull addxemmm/gsm-system:2.1
# Exact version / 固定版本
docker pull addxemmm/gsm-system:2.1.0
# For a digest-pinned pull, copy the command from the GitHub Release.
# 按不可变 digest 拉取可复制 GitHub Release 中的命令。
```

Pulling does not start a container. Deployment still uses existing gates and site
configuration, preserving the external business volume. Default Web is 18082;
independent API 8082 requires the overlay. Publication does not change the current
test server's dual-port settings. See [deployment](DEPLOY.md) and [Web](WEB-CONSOLE.md).

拉取不启动容器；部署仍走既有门禁与站点配置，保留外部业务卷。默认 Web 18082，独立 API
8082 需 overlay；发布不改变当前测试服务器双端口设置。

## Publication contents / 分发内容

Seed databases are reconstructed before entering the final image, excluding
subscriber/message data and deleted SQLite pages. Native notices and a
corresponding-source bundle accompany publication, including pinned sources,
local compatibility changes and build instructions, not live operator data.
Checks cover explicit structures/allowlists, not a universal secret-free or
license-compliance certification.

种子库先重建再进入最终镜像，排除用户/短信数据与 SQLite 已删除页。发布带原生许可
声明及对应源码包，包含固定源码、本地兼容修改和构建说明，不含现网数据。结构检查
不等同全面无密或法律认证。

OpenBTS, UHD, Asterisk and firmware have their own terms; project MIT does not
replace them. Clone-board firmware has vendor provenance but no included
redistribution grant. See [third-party inventory](THIRD-PARTY.md).
OpenBTS、UHD、Asterisk 与固件各适用自身条款；克隆板固件有来源记录但项目未附供应商
再分发授权文本，不将其冒充许可认证。

## Legacy tooling / 旧手动工具

`go run ./scripts/release` remains for historic server-image promotion and digest
verification. Its internal revision tags/manual flags are **not** the normal
new-release path. Use the workflow and tag helper above.
旧 Go 工具保留历史手动镜像核验兼容性，其内部 revision 标签与人工参数不是新发版入口。
