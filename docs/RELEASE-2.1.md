# Release 2.1 acceptance record / 2.1 发布验收记录

Version / 版本：`2.1.0`

## Latest deployment: 2026-09-09 12:37 +08:00 / 最新部署：双语 Web 管理台

- Runtime / 运行镜像: **`gsm-system:2.1`**, source revision **`8d02b2b670b1`**.
  Image ID / 镜像 ID: `sha256:51807bbde23dcb9826309aca6495e603d5ae84c715a7a828eda382c95d4d010a`.
  Subsequent acceptance-only documentation commits do not change this binary.
  后续仅验收文档的提交不会改变此运行程序身份。
- The Go binary embeds the bilingual responsive console and serves Web plus the
  same authenticated API in one container. Dashboard/process topology, three
  explicit start sources, stop, presets, bindings, SMS, calls, NAT and field
  guidance are available. / Go 内嵌双语响应式前端，同容器提供页面与统一 API；
  覆盖仪表盘/进程拓扑、三种显式启动、停止、预设、号码绑定、短信、通话、NAT 与指南。
- Default deployment publishes only Web `8080`, with the independent API bound
  to container loopback. This test installation explicitly enables both `8080`
  and `8082` on its LAN address. Both health endpoints return the same version
  and revision; optional token is currently blank. Bridge mode, `restart=no`
  and `Asia/Shanghai` (`+0800`) remain intact. / 默认仅 Web，测试站点显式开放
  两端口，双入口版本一致；当前令牌留空，保留 bridge、不自启及东八区。
- Windows Go tests/vet, Linux race/vet, Windows/Ubuntu deployment contracts,
  Postman contracts and 11 browser-source checks passed. The source commit's
  [GitHub CI](https://github.com/addxemmm/gsm-system/actions/runs/34311325333)
  passed Go 1.22, Go 1.26.8 and Windows jobs.
  Windows/Linux Go、竞态、部署、Postman 与 11 项前端检查通过，云端 CI 全绿。
- All five final-image suites passed: caller ID/2600/2602 isolated routing,
  runtime persistence/timezone/auth/CDR, current-start SMS and deduplication,
  presets, and Web/API Compose/auth/Origin/health. These tests do not transmit RF.
  五组最终镜像隔离测试全部通过，含来显/测试号码、持久化/时区/鉴权/话单、
  当次短信与去重、预设、Web/API 端口合并和同鉴权；未进行射频发射。
- Real-browser checks covered Chinese/English, 390-pixel responsive layout,
  preset creation/deletion, confirmation followed by Escape cancellation,
  failed deletion with request ID, token clearing/reload and draft retention
  across refresh/language changes. No browser console errors were observed.
  浏览器实测双语、390px、预设增删、确认后 Esc 取消、失败提示、清令牌与草稿保护，
  未观察到控制台错误；写入测试仅使用无业务卷、无 USB 的预览容器。
- Before/after recreation, subscriber SQL, OpenBTS configuration SQL and raw
  smqueue log digests matched. SQLite integrity was OK; all three subscribers
  and three number bindings remained. LTE identity and start/finish timestamps
  were unchanged. / 重建前后签约库、OpenBTS 配置及短信原始日志摘要一致，
  数据库完整，三用户三绑定保留，LTE 身份与起止时间未变。
- Removed preview/old GSM containers, superseded GSM images, the exact warmed
  builder image and unused build cache. Final inventory: current GSM and LTE
  only, two business volumes retained, build cache **0 B**.
  清理预览、旧 GSM、预热构建镜像与无用缓存；最终仅 GSM/LTE 两镜像、两容器，
  两个业务卷保留，构建缓存 **0 B**。

**The management container is healthy; the cell was intentionally left stopped
after the upgrade.** Start from the Web cell-control page when ready. Existing
handset GPRS reliability findings below are not claimed fixed by a UI release.
**管理容器健康，小区在升级后明确保持停止。** 可从 Web 小区控制页显式启动；
本次不将前端升级或管理健康误称为下文手机 GPRS 可靠性问题已解决。

Docker Hub publication tooling and manual release-record gates are prepared;
no Hub upload, Git tag, Release or secret was created. Destination, visibility,
image-content/licensing review and credentials remain operator setup steps.
Docker Hub 发布工具与手动发版门禁已准备，未上传、建 tag/Release 或配置密钥；
仓库目标、可见性、镜像内容/许可证审核及凭据仍须由操作者确定。
See [Web tour](WEB-CONSOLE.md) and [release procedure](RELEASING.md).

## Follow-up: PDP established, radio reliability trial / 后续：PDP 已建立，继续无线可靠性对照

At approximately 10:16–10:24 +08:00 on September 9, a handset reached registered
SGSN state and obtained PDP addresses. TUN and NAT counters increased in both
directions; bounded header-only inspection confirmed upstream DNS responses.
Browsing was still reported unavailable. CS1-only coding improved the observed
active transfer; idle downlink assignment failures remained, so per-MS slot caps
were subsequently set to 1/1 pending reattachment and actual browser acceptance.
See [GPRS recovery](GPRS-RECOVERY.md) for values, original values, and limitations.
No code or image changed in this follow-up: runtime remains `19a03406d30e` /
`gsm-system:2.1`, with container auto-start disabled. Temporary packet-diagnostic
containers automatically removed themselves; no packet files were saved and no
LTE object was changed by this task.
9 月 9 日约 10:16–10:24（东八区），手机已注册并取得 PDP 地址，隧道/NAT 有双向流量，
限时包头检查确认上游 DNS 回包；用户仍反馈网页失败。仅用 CS1 后活跃传输改善，但
空闲态下行分配仍失败，继而设置每手机 1/1 时隙上限，等待重新接入及实际网页验收。
配置、原值及边界详见上方文档。本次仅改站点运行配置和文档，未改变代码或镜像，
仍为 2.1 / 19a03406d30e，关闭自启；诊断容器已自动删除，无抓包文件，本任务未动 LTE。

## Previous deployment: 2026-09-09 09:56 +08:00 / 历史部署

This historical summary describes the earlier radio-fix deployment. The latest
Web deployment identity is recorded above; older sections are not current inventory.
本节为此前无线修复部署历史；最新 Web 部署身份见上方，不将旧记录视为当前库存。

- Runtime / 运行版本: `gsm-system:2.1`, binary revision `19a03406d30e`.
- Image / 镜像: `sha256:b5c0523289d5ce5d23b9b5df79a5e3dcbc6c94820e4353be77ebaf4ecbd50d31`.
- Fixed the pinned public pager's pre-IMSI GPRS assignment early return; the
  original-source negative control reproduces the defect. Authentication and
  CS paging are unchanged. / 修复尚未取得 IMSI 时提前丢弃 GPRS 下行分配的问题，
  原始源码失败对照可复现；鉴权与普通语音寻呼不变。
- Explicit cell start now restores the exact missing GSM NAT rule before native
  launch. The recreated container had no GSM rule; restoring its saved profile
  added it without a separate network PUT. `rule_present=true` and
  `ipv4_forwarding=true` were observed. / 新容器原本没有 GSM NAT，显式恢复保存配置
  后自动补齐，未手动调用网络写接口；规则与转发状态均已核验。
- Windows Go tests/vet, Linux Go race tests/vet, Postman offline contracts,
  deployment/build fixtures, native positive/negative regressions, and all four
  isolated image suites passed. Caller-ID, 2600/2602 local dialplan, persistence,
  timezone, SMS scope, NAT recovery and preset tests remain green.
  Windows/Linux Go、竞态、Postman、构建部署样本、原生回归及四组隔离镜像测试均通过；
  来电显示、测试号码本机路由、持久化、时区、短信范围、NAT 与预设未发现测试回归。
- Subscriber SQL dump and raw SMS log hashes matched before/after recreation;
  SQLite integrity was OK, with three subscribers and three number bindings.
  LTE container/image identity and start/finish timestamps were unchanged.
  容器重建前后签约库导出和原始短信日志摘要一致；数据库完整，三用户三号码绑定保留；
  LTE 容器、镜像和运行时间戳未变。
- The cell was explicitly restored at `2026-09-09T09:56:26+08:00` and reported
  running/ready, SMS-ready and voice-ready. Container restart policy remains `no`.
  小区已显式恢复，管理状态全部就绪；容器开机自启仍关闭。
- Old GSM objects, unused Go/Ubuntu build-base images and build cache were
  removed. Only the current GSM and untouched LTE images remain; business
  volumes were retained. / 旧 GSM 对象、无引用构建基础镜像与构建缓存已清理，
  仅保留当前 GSM 与 LTE 镜像，业务数据卷保留。

**Handset Internet acceptance is pending reattachment.** At the first post-start
snapshot SGSN had no handset context and the TUN had no received packet. This
is not evidence that a phone has obtained an IP or accessed the Internet; the
operator must reconnect a test phone and verify PDP, DNS and an actual page.
**手机上网仍待重新接入验收。** 首次启动后快照没有手机 SGSN 上下文，TUN 尚无接收
流量，不将小区就绪或 NAT 恢复等同于手机已上网；需重新接入并验证 PDP、DNS 与网页。

An additional RF observation found repeated RACH clipping with B200 RX gain at
47 dB. The pinned configuration description recommends 0–10 dB for Ettus rather
than the 47 dB RAD1 default. RX gain was temporarily changed to 10 dB through
the native CLI; the subsequent checked log interval contained zero new clipping
alerts and the cell remained ready. TX power and persistent configuration were
not changed. This short observation does not establish handset coverage or data
service quality; recheck with the actual test phones before making it persistent.
另发现 B200 接收增益沿用 RAD1 的 47 dB 默认值并反复削顶；内置参数说明给 Ettus 的
参考范围为 0–10 dB。通过原生 CLI 临时降至 10 dB 后，检查区间新增削顶告警为零，
小区仍就绪；未改变发射功率或持久配置。短时观察不代表手机覆盖或数据质量验收，
应先结合测试手机确认，再决定是否持久化。

The native TUN reader also lacks an IP-version guard. Four initial 48-byte
packets were consistent with IPv6 router solicitation being parsed as IPv4,
but no packet capture confirmed that interpretation. The missing-PDP branch
only drops the current packet; it does not stop the reader. Do not interpret
those destination strings as confirmed handset traffic or disable IPv6 globally.
原生 TUN 读取器还缺少 IP 版本检查：启动时四个 48 字节包与 IPv6 RS 被误作 IPv4
解析的现象吻合，但未经抓包确认。找不到 PDP 的分支仅丢当前包、不停止读取；
不要据此把日志目的地址认定为手机流量，也不要全局禁用 IPv6。

## Release identity / 发布身份

The Go management plane was deployed to the SDR server between
**2026-09-08 10:45 and 10:50 HKT** from source revision `16986725daa9`.
The immutable image is `gsm-system:2.1.0-16986725daa9`; the validated release
alias is `gsm-system:2.1.0`. Both resolve to:

```text
sha256:1dab7a283f07e0017f1f1a3a450f7498f55a02d69b8c34101ea26082efdecf5a
size: 406 MB
```

2.1 Go 管理面已于上述时间从 `16986725daa9` 部署；不可变标签与发布标签指向同一
406 MB 镜像 ID。OCI version/revision 和 `gsm-system --version` 已核对一致。

Runtime / 运行环境：Ubuntu 22.04, Asterisk 18.10, SQLite 3.37.2. Compose
project is `gsm-system-live`, service `gsm-system`, active container
`gsmsystem-uhd4`, external volume `docker_gsm-data`. The retained pre-2.1
rollback container is `gsmsystem-rollback-pre21-20260908`.

## Delivered scope / 已交付范围

- The project-owned HTTP/process-management plane is Go-only. OpenBTS, UHD, and
  Asterisk remain native software; upstream UHD still uses Python/Mako at build time.
- Public routes exist only under `/api/v1`; retired root routes return `404`.
- Connections and subscribers are distinct; number binding is transactional and
  unique; SMS returns `202 submitted`, never a delivery claim.
- Calls use real Asterisk active channels; history maps the configured 18-column
  CSV CDR and normalizes GMT timestamps to RFC3339 UTC.
- Network configuration is `GET` plus idempotent `PUT`, reports
  `persisted:false`, and observes but does not alter IPv4 forwarding.
- One production Dockerfile/Compose definition, versioned images, persistent
  native databases/CDR, and a Go `/dev/log` collector replace the split legacy line.

自研管理面已完成 Go 化和版本化 API 收敛；射频栈仍为原生 C/C++，未声称整套射频
软件被重写。日志、原生数据库、CDR 与外部数据卷具备明确持久化边界。

## Recorded acceptance / 已记录验收

Native and full runtime image builds passed. `scripts/tests/test_image.sh` passed
against a new disposable volume with `--network none`, no USB device, no
privilege, and therefore no RF path. It verified image identity/provenance, the
Go syslog socket, ODBC/CDR modules and dialplan loading, database/CDR persistence
across restart and container recreation, and clean fixture ownership/removal.

A real local Asterisk bridge exercised `GET /api/v1/calls`; a completed local
fixture produced a real 18-column `ANSWERED` CDR consumed by
`GET /api/v1/calls/history`. This test used no handset, SIP endpoint, or RF.
本地 Asterisk 桥接与真实 18 列话单只验证管理面/本机语音数据链路，不代表空口通话。

The final-image Newman run completed **12 requests and 32 assertions with zero
failures**. Every mutation request was skipped because
`enable_mutations=false`; no cell start, config write, binding, SMS transmission,
or network mutation occurred. OpenAPI passed Redocly strict validation with zero
warnings. Windows full tests/vet and Linux full race/vet passed. GitHub CI for
revision `16986725daa9` also passed:
[run 34181076912](https://github.com/addxemmm/gsm-system/actions/runs/34181076912).

A deployment-script orphan-handling correction was committed separately as
`8d361a0` and rerun successfully on the server. It did not change the runtime
source identity: the deployed image remains revision `16986725daa9`, and the
`gsm-system:2.1.0` alias was reconfirmed against that same image ID.

The active management container is healthy on LAN port `8082` with token auth
intentionally unset for this LAN deployment. `GET /api/v1/cell` reported every
managed process false and RF off. LTE container ID, state, start time, and finish
time were identical before and after GSM deployment. 管理容器健康，小区进程全部停止，
部署未启动射频；LTE 状态在部署前后完全未变。

All four native database backups passed integrity checks, and logical-dump hashes
matched before/after deployment. Evidence, backups, and logs are retained at:

```text
$HOME/gsm-release-2.1-20260908T014046Z
```

Cleanup removed six obsolete/candidate/test/build tags and all 354 build-cache
entries (16.17 GB to zero). Images went from 14 to 7 while keeping one image ID
for the two 2.1 tags. Three containers remain: current GSM, pre-2.1 GSM rollback,
and LTE. Two data volumes and the LTE rollback image were retained. Disk free is
38 GiB with 49% used. 清理未删除线上/回滚容器、GSM/LTE 数据卷或 LTE 回滚镜像。

## Remaining acceptance and risk / 待验收与风险

**Not yet verified / 尚未验证：** handset network attach, over-the-air SMS,
two-way handset voice, RF behavior/quality, and full RF-chain acceptance. Release
2.1 management-plane deployment must not be described as successful handset or
RF acceptance. 尚不得宣称手机入网、空口短信、双向手机语音或射频业务实机通过。

Asterisk still uses the native legacy `chan_sip` stack. This is a documented
technical-debt/risk boundary, not a claim of replacement by a modern SIP driver.
Asterisk may emit autoload warnings for non-critical optional modules; such
warnings must be reviewed, but do not by themselves mean the explicitly verified
ODBC, CDR, dialplan, or core modules failed. Asterisk 仍保留原生旧 `chan_sip`；非关键
可选模块的 autoload 警告不等于已验证核心模块失败。

For upgrade and rollback procedure, see [`MIGRATION.md`](MIGRATION.md) and
[`DEPLOY.md`](DEPLOY.md). Preserve the immutable image, named rollback container,
database backup set, and evidence directory until handset/RF acceptance closes.

## Post-acceptance operator update / 验收后运维更新

On 2026-09-08 the operator adopted a current-version-only retention policy.
The old GSM images, stopped rollback container, old release/audit packages, and
old host backup directory were removed; the active GSM image ID with both tags,
active container, and external business-data volume were retained. This latest
operator decision supersedes the earlier retention recommendation above for the
current host, without rewriting the historical acceptance record.

2026-09-08 运维改为仅保留当前版本：旧 GSM 镜像、停止的回滚容器、旧发布/审计包及
主机旧备份目录已删除；在用 GSM 镜像 ID 及两个标签、当前容器和外部业务数据卷均保留。
此最新决定取代上文针对当前主机的旧保留建议，但不改写历史验收记录。

An isolated, non-RF Compose fixture then verified all optional-token modes with
no host token export: non-empty `.env` made unauthenticated/wrong-token requests
return 401 and the matching token return 200; blank or missing `.env` left
authentication disabled and returned 200. Deployment health passed in every
mode, the token did not appear in captured deployment logs, fixture resources
were removed, and the live container was unchanged.

随后以无射频、无 USB、非特权的独立 Compose 样本验证可选令牌：宿主 shell 未导出令牌；
`.env` 非空时无令牌/错误令牌返回 401、正确令牌返回 200；留空或缺少 `.env` 时鉴权关闭并
返回 200。三种模式的部署健康检查均通过，采集日志未出现令牌，样本资源已清理，线上容器未变。

## Named presets and bridge networking / 命名预设与桥接网络

- Add user-owned `/api/v1/presets` CRUD and `POST /api/v1/cell` with
  `preset_id`; retain explicit starts and `{}` last-profile reuse. There are no
  built-in carrier presets and no restored legacy root routes.
  新增用户预设增删改查及按 ID 启动，保留显式启动与上次存档复用；不恢复旧运营商预设和旧接口。
- Persist complete presets atomically in `/data/presets.json`, mode `0600`,
  with bounded input/storage and fail-closed corruption handling. Preset edits
  never implicitly change or start a cell. 预设限量、原子持久化；损坏时拒绝覆盖，编辑不触发小区变更。
- Use one Docker bridge and the stable container uplink `eth0`, publish only
  TCP 8082, and keep the native SIP/RTP/CLI/TRX traffic inside the container.
  改为单 bridge、固定容器出口 `eth0`，仅映射管理面端口；原生协议端口不对外发布。
- Update bilingual API/operational documentation, OpenAPI and guarded Postman
  requests together. Add management-only PowerShell smoke tests and an isolated
  image test with a disabled hardware detector. 同步中英双语文档、接口契约和测试文件；
  测试区分真实管理 API 验收与尚未执行的手机/射频全链路验收。

### Deployment acceptance, 2026-09-08 / 本次部署验收

The active runtime now supersedes the earlier `16986725daa9` deployment:
当前运行版本取代上文旧部署，但保留历史记录：

| Evidence / 证据 | Result / 结果 |
|---|---|
| Runtime source / 运行源码 | `36af25acf8db` (preset feature / 预设功能 `4d7a73e`) |
| Immutable image / 不可变镜像 | `gsm-system:2.1.0-36af25acf8db` |
| Release alias / 版本别名 | `gsm-system:2.1.0`, same image / 同一镜像 |
| Image ID / 镜像 ID | `sha256:02a82de72e1db03519d184cd605883807a763e85dc7ee4800499a5097edea497` |
| Container / 容器 | `gsmsystem-uhd4`, healthy / 健康 |
| Network / 网络 | `gsm-system-live_gsm-bridge`, bridge driver, `eth0`, `172.31.240.0/24` |
| Published ports / 映射端口 | TCP 8082 bound to the operator's LAN address only / 仅绑定用户局域网地址 |
| Authentication / 鉴权 | `.env` Token remains empty; no Token needed / 保持空值、免令牌 |
| Cell / 小区 | `stopped`; all five native process flags false / 五个原生进程均未启动 |

Acceptance performed / 已执行验收：

1. Reverified pinned native sources and rebuilt UHD/OpenBTS and the complete
   runtime through the official deployment script. The discovered zsh `NOMATCH`
   sync failure was fixed by explicit POSIX `sh` stdin execution, with Windows
   regression and a real-server check. 重新校验依赖并完整构建；修复真实服务器
   zsh 通配符展开导致的同步失败，完成脚本回归与服务器验证。
2. The isolated image test passed preset CRUD, omitted descriptions, file mode
   `0600`, restart persistence, and both start modes. Its detector was fixed to
   `/bin/false`; both valid starts returned `503`, with no USB, privileges or
   published ports. 独立镜像测试通过预设持久化与双启动路径；硬件检测固定关闭，
   未将此测试描述为真实小区启动成功。
3. LAN PowerShell smoke passed CRUD, duplicate `409`, missing `404`, mixed
   selector/invalid update `422`, and cleanup. The final preset list was empty:
   no test records or old carrier defaults remain. 局域网冒烟通过，测试配置已删除，
   未导入旧运营商默认配置。
4. Migrated only saved-profile `network` from the host NIC to `eth0`. OpenBTS
   and subscriber SQL-dump SHA-256 digests matched before/after, and LTE identity,
   image, state and timestamps were unchanged. 仅迁移存档网卡；配置及签约数据摘要、
   LTE 状态均保持一致。
5. Two stopped-state `PUT /api/v1/network` calls returned `changed:true` then
   `false`; forwarding and the rule were true. `persisted:false` remains the
   documented contract: reapply after recreation. 停止态 NAT 连续设置验证幂等；
   容器重建后仍需重新执行网络 PUT。
6. Removed the superseded GSM image and temporary native builder image after
   acceptance, then pruned unused build cache to `0 B`. Only the current GSM
   image ID and its two aliases remain; business volumes and LTE were preserved.
   验收后删除旧镜像与临时构建镜像、清空构建缓存，保留业务卷和 LTE。

Both feature and runtime-source GitHub CI runs passed. RF attach, handset SMS
delivery and live voice testing were not performed in this management-only
release. 两次功能/运行源码 CI 均通过；此次仅验收管理面，未执行射频入网、手机
短信送达或实时通话测试。

## Built-in defaults correction / 内置默认预设更正

The operator clarified that the five configurations must ship inside the system,
not require manual creation. This decision supersedes the earlier empty-initial-
list/no-defaults behavior recorded above. 用户明确要求系统内置下列五套配置，
无需逐条手动添加；此要求取代上文记录的“初始为空、不内置预设”行为。

| ID (string / 字符串) | Name and short_name / 名称 | Band / 频段 | C0 | MCC/MNC | LAC/CI |
|---|---|---|---|---|---|
| `0` | addx | 1800 | 540 | 001/01 | 1/1 |
| `1` | ChinaMobile | 900 | 55 | 460/00 | 1/1 |
| `2` | ChinaMobile | 1800 | 540 | 460/00 | 1/1 |
| `3` | ChinaUnicom | 900 | 70 | 460/01 | 1/1 |
| `4` | ChinaUnicom | 1800 | 668 | 460/01 | 1/1 |

All use `arfcns="1"` and `network="eth0"`. New stores initialize these defaults.
Existing version-1 stores merge only missing default IDs and migrate atomically
to version 2 without replacing user-owned collisions. Version-2 files remain
authoritative: subsequent edits/deletions survive restarts. Corrupt or oversized
migrations fail without replacing the source. 全部使用单载波及 `eth0`；旧存储
一次性补齐缺失默认项，保留用户同 ID 配置；后续修改和删除不会在重启时被撤销。
损坏或超限迁移不会覆盖原文件。

The versioned preset API, explicit starts and last-profile reuse remain; legacy
root routes are still retired. Initialization never starts RF or rewrites the
active radio/subscriber databases or last-start profile. 保留新预设 API、显式启动
与上次存档复用，不恢复旧根路径接口；初始化不启动射频、不改现有小区及签约数据库
或上次启动存档。

### Deployed defaults acceptance / 内置预设部署验收

On 2026-09-08, runtime source `fbdaa5be2209` was built through the official
Windows-to-Ubuntu deployment script and deployed as
`gsm-system:2.1.0-fbdaa5be2209` (alias `gsm-system:2.1.0`), image ID
`sha256:9b14b87dd4022d7f67674030c5d0c3c43703dc05adeae6f5ee246496684d07a5`.
2026-09-08 已通过正式脚本构建并部署上述源码与镜像；版本别名指向同一镜像。

- The existing empty version-1 store automatically became version 2 with the
  exact five defaults above, mode `0600`; list and individual GET requests
  matched every field. 现有空预设库自动升级，五项全部字段及逐项查询验收通过。
- The isolated image test passed initial defaults, CRUD, restart persistence,
  non-respawn after deletion and both start paths with hardware detection
  deliberately disabled. The live API CRUD smoke test removed only its own
  unique fixture. 独立镜像测试和在线 CRUD 冒烟通过；模拟启动未访问射频，测试
  数据已清理，线上五套默认项完整保留。
- The management container is healthy on the dedicated bridge, publishing
  only LAN-bound TCP `8082`, with Token left blank. RF/native services remain
  stopped. Container-local `eth0` NAT was reapplied and the second request
  returned `changed=false`. bridge、局域网免 Token、射频关闭均保持；容器内
  NAT 已恢复且重复调用验证幂等。
- Radio/subscriber SQL dump hashes, last-start profile hash and the LTE
  container identity/image/stopped timestamps were unchanged. 小区、签约数据库、
  上次启动配置及 LTE 容器状态均未改变。
- Superseded GSM and temporary native-builder images were removed after the
  new image became healthy. Build-cache cleanup reported `3.519GB` reclaimed;
  final build cache is `0B`. Only the current GSM image and its two aliases
  remain; LTE, data volumes and necessary base images are retained. 已清理旧
  GSM 镜像、临时构建镜像及缓存，不保留回滚；LTE 和业务数据卷保持不变。
- Windows unit/vet and deployment tests, Linux race/vet and build contracts,
  and GitHub CI for Go 1.22/1.26.8 plus Windows passed
  ([runtime CI](https://github.com/addxemmm/gsm-system/actions/runs/34186908980)).
  本地及 CI 验证通过；此次仍未执行真实手机入网、短信送达或实时通话测试。

Later documentation-only commits do not change the deployed runtime revision.
后续仅文档提交不改变服务器记录的运行源码版本。

## Start-mode usability / 双启动模式易用性

The existing management API already accepts a stored `preset_id` or a complete
custom profile, with `{}` retained as a distinct saved-profile reuse mode.
This update keeps that runtime behavior and clarifies the client workflow; no
image rebuild or service restart is required. 管理 API 已支持预设和完整自定义
配置，并保留独立的 `{}` 存档复用模式；本次完善客户端和回归验证，不改变
运行逻辑，因此无需重建镜像或重启服务。

- Postman exposes separate, clearly named preset/custom requests in a dedicated
  cell-start folder. `start_preset_id=0` is independent of read-only default
  lookup and CRUD fixture IDs. 所有预设均可按 ID 启动，启动变量与查询/测试变量隔离。
- Custom `arfcns`, `band`, `short_name` and the remaining parameters are all
  variables; the default short name is `addx`. 自定义九项均可配置，名称默认 `addx`。
- Both explicit start switches still default to false. Missing switches and
  unresolved/mixed/incomplete input produce a bilingual Console diagnostic
  before skipping. Empty/false environment values override collection values,
  including an empty optional Token. 双开关保持默认关闭；前置检查不再静默跳过，
  并按变量优先级正确处理空值和 false。
- Offline Node sandbox tests execute the exported scripts without HTTP/RF and
  are included in Windows CI. API regressions exercise start-mode isolation
  against an absent hardware detector, not an actual cell launch. 新增脚本离线
  执行验证和无硬件 API 模式隔离回归，不执行真实射频启动。

### Direct manual Send / 手动直接发送

The operator requested removal of the extra Postman enable switches. This
supersedes the client-switch behavior recorded above; they were never backend
API requirements. Both `enable_mutations` and `enable_rf_start` are removed from
the collection, environment and request scripts. Valid manual requests now send
directly, including preset/custom starts and other writes. 用户要求移除额外的
Postman 启用开关；此决定取代上文历史行为。两个开关原本就不是后端 API 要求，
现已从集合、环境和脚本移除，有效请求点击 Send 即执行。

Input validation and its bilingual Console diagnostics remain, as do optional
Bearer auth and the independent factory-response assertion option. Old imported
environment switch values no longer affect sending; re-import the new collection
to replace the old scripts. 参数检查、双语 Console 提示及可选 Token 保留；旧环境
中残留的开关不再生效，需重新导入新集合替换旧脚本。工厂响应断言选项不影响发送。

Use the GET-only folder for read-only inspection. Running the entire collection
can start/stop the cell, delete records and submit SMS; prefer individual Send.
Offline script tests verify direct sending even with stale false switches and
still reject malformed start input. This is a client/docs/test-only change, with
no image rebuild, service restart or RF operation. 只读检查仅运行 GET 分组，
不要全量运行包含实际写操作的集合；离线测试覆盖旧开关残留及参数拒绝。本次不重建
镜像、不重启服务、不操作射频。

## Registration observation correction / 注册观测纠正

A live, read-only investigation found three rejected TMSI observations, not
three successful registrations: native `AUTH=0`, `REJECT_CODE=4`, no assigned
TMSI, an empty subscriber/routing registry, and repeated registrar `imsi unknown`
messages. `GPRS.Enable=0`, the SGSN list was empty, and the container's `eth0`
forwarding/NAT rule was present. 当前只读调查确认列表为被拒绝的注册尝试；GPRS
未启用，空 IP 不是 NAT 故障的证据。没有保存真实终端标识或密钥。

- Fix a parser defect that collapsed blank identity columns and misread
  `WELCOME_SENT=1` as a telephone number. Parse the native fixed-width header,
  and derive numbers only from explicit telephone identities. 修复空列折叠造成
  的号码错位，不再将欢迎短信状态当作电话号码。
- Recognize native SGSN `IPs=` as well as legacy `IP=`, validating address lists
  rather than displaying arbitrary tokens. 支持真实复数 IP 字段及有效地址列表。
- Add nullable raw `auth` / `reject_code` diagnostics to connection responses.
  They describe the stored observation, not real-time online status; unknown
  values remain null. OpenAPI/Postman permit absent fields on older 2.1 runtimes
  during rollout. 新增诊断值兼容旧版响应，不将记录存在误判为成功入网。
- Document strict registration versus OpenBTS open registration, PLMN selection,
  packet-data versus signaling SMS, and the fact that `WELCOME_SENT`/HTTP `202`
  do not prove delivery. 正常注册欢迎消息为空时不发送；曾经调度失败提示短信也
  不代表正常欢迎短信送达。

Synthetic parser/API regressions, Windows unit/vet, Linux race/vet and offline
Postman script tests passed. This investigation did not send SMS, provision SIM
keys, enable open registration/GPRS, restart the cell or replace its image.
At that investigation stage the running image remained source `fbdaa5be2209`;
deployment and access-policy changes awaited operator confirmation. 合成回归验证
通过时仅修复源码和契约，尚未变更运行镜像；后续经确认部署的结果见下节。

## Registration/data deployment verification / 注册与数据配置部署验收

After operator approval on 2026-09-08, stopped the cell and old management
container, rebuilt committed source `08f184f063c5`, and deployed
`gsm-system:2.1.0-08f184f063c5` (validated alias `gsm-system:2.1.0`).
Image ID: `sha256:ac59eb214fb023b4022796fb5b5b15723e76ea23ed8fa926bd404c525e0cac05`.
用户确认后停止旧小区及容器，完整重建并部署上述修复版本，保持射频关闭，由用户
手动启动及重新接入测试设备。本节及接入指南为后续文档变更，不改变镜像源码版本。

- Persisted an explicit open-registration policy for 15-digit IMSIs, cleared its
  reject pattern, and set both open/normal welcome messages to the addx greeting
  with shortcode `101` and frequency `FIRST`. 持久配置显式开放注册匹配规则、空
  拒绝规则及两类欢迎短信；未写入 SIM 密钥或凭空创建签约/号码。
- Enabled GPRS with minimum C0/CN channels `2/0`; preserved automatic signaling
  allocation and the existing GGSN pool. Configured the host's actual upstream
  LAN DNS rather than Docker's loopback resolver. 启用 GPRS，保留信令时隙与地址池，
  下发实际上游 DNS；真实服务器地址未写入此文档。
- Retained the dedicated bridge and only the LAN-bound TCP management port.
  Reapplied the `eth0` NAT rule: first PUT `changed:true`, second PUT
  `changed:false`, both with forwarding/rule present. 保持 bridge 与必要管理端口；
  容器内转发及幂等 NAT 验证通过，Token 仍为空、免鉴权。
- Management health and image/version checks passed; cell stayed `stopped` and
  connections correctly returned HTTP `412`. OpenBTS/TMSI/subscriber database
  checks passed; volatile TMSI count was zero, persistent subscriber/routing
  counts remained unchanged at zero. 管理面正常，所有射频/业务进程保持停止，易失
  注册记录已重置，未清除持久业务库。
- Isolated image tests passed for five defaults, CRUD/persistence and both start
  paths, with no USB, privileged access or RF. Removed the old GSM image after
  validation; build-cache cleanup reported 3.524 GB, with zero cache remaining.
  镜像隔离测试通过，旧容器已替换、旧镜像已删除，不留回滚；LTE 容器、镜像及卷
  未变更，保留必要的 Go/Ubuntu 构建基础镜像。

The pre-deployment native log also recorded `UHD: Receive timed out` followed by
transceiver/OpenBTS exit. USB enumeration still showed the SDR, but no radio
restart or handset delivery/PDP test was performed by this deployment. This
hardware/runtime issue is not declared fixed. See SIM.md for the reattach/APN,
welcome/manual SMS and data acceptance steps. 部署前另有 UHD 超时退出证据，USB
枚举仍有设备；本次不将管理面验收当作射频、手机短信实收或 PDP 成功，需用户重接
设备后验证，超时复发时再检查 USB 直通及射频运行日志。

## SMS onboarding and history correction / 短信注册与历史查询修正

The operator confirmed welcome delivery and open-registration observations.
Read-only diagnostics then found three consistent persistent number bindings
despite missing confirmations: Asterisk rejected the local OpenBTS contact and
smqueue received `603` during the subsequent registration stage. 用户已确认欢迎
短信实收；三个自选号码已在两个权威表中一致入库，后续注册阶段却被 Asterisk
联系人 ACL 拒绝，并非用户回复没有到达服务器。

- Ship a dedicated Asterisk contact ACL that denies IPv4/IPv6 by default and
  permits only `127.0.0.1/32`, included under the legacy `[general]` section with
  CRLF-compatible insertion. The live SIP-only reload preserved the running
  cell. 镜像固化最小本机联系人许可，不开放全部局域网联系人，不修改用户未提交
  的 `modules.conf`。已对运行实例做 SIP 热重载，小区未因此重启。
- Prefer consistent persistent number bindings in `/connections`, with an
  explicit `number_source`; authoritative unbinds override stale TMSI values.
  连接列表优先显示已入库号码，明确来源，异常绑定不伪装成有效号码。
- Restore production NOTICE history parsing. Identity-based deduplication merges
  retries without deleting legitimate repeated text; ambiguous unkeyed text
  stays unknown. Add a pinned native `GSM_SMS_V1` receive-observation record with
  hex-encoded qtag/from/to/text, so future concurrent events carry their own
  addressing and text. 修复 NOTICE 日志漏解析与去重；新原生日志逐事件携带明确
  身份及字段，hex 防止正文注入伪日志行；Go 负责查询及归一化，未引入 Python。
- Welcome/onboarding guidance describes native `101` registration (7–10 digits),
  `411` lookup and API number changes. 欢迎提示与短信注册的实际长度和回执行为一致。

The GPRS inspection found a present TUN route, forwarding and NAT, successful
container DNS/HTTP, and one packet-attached handset. It did not establish
handset web access; no speculative firewall disabling or route replacement was
performed. 网络检查已有 TUN、转发/NAT、容器 DNS/HTTP 可达及一个数据附着终端，
尚未证明手机网页成功；保留已有规则，等待主动手机流量与上下行计数对照。

## Follow-up audit, 2026-09-08 / 后续审计

This is an audit and code-correction record, **not a new deployment record**.
本节记录现场审计及代码修复，**不代表新版已部署**。

- Observed runtime: `gsm-system:2.1.0-08f184f063c5`, binary revision
  `08f184f063c5`. The latest earlier local SMS commit `e2d65066cc9e` was not
  installed. GitHub branch inspection still returned `45842fb` before this audit
  was committed. / 现场运行旧镜像；前次短信修复尚未安装，审计时 GitHub 分支亦落后。
- The GSM API reported `state=degraded`, `smqueue=false`, while Docker reported
  healthy. Three subscriber and three number-routing rows remain present.
  / 小区处于降级状态、短信队列退出，但旧 Docker 探针仍报健康；三条签约及号码路由仍在。
- There is only one GSM image ID with two aliases, no stopped GSM rollback
  container, and **0 B build cache**. The stopped LTE container/image and the
  Ubuntu/Go build bases remain intentionally. Vendor inputs (~230 MiB) are
  necessary pinned build sources, not disposable generated cache.
  / 无旧 GSM 镜像或回滚容器；构建缓存为零。保留 LTE、构建基础镜像及必要上游源码。
- Follow-up fixes add a read-only state-aware Go health probe, migrate known
  legacy welcome defaults before native startup while preserving custom/blank
  messages, retain process exit metadata without SMS contents, and align custom
  SMS log filenames between collection and the history API.
  / 本轮补齐真实状态探针、启动前默认欢迎文案迁移、无正文的退出诊断及短信日志文件名一致性。
- Build scripts now use the root `.env` explicitly and preflight the volume
  selected by Compose interpolation rather than a conflicting shell-only value.
  No Token is sourced/evaluated or printed. See [Docker interpolation precedence](https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/).
  / 构建显式读取根环境文件；数据卷预检遵循 Compose 实际插值，不输出或执行令牌内容。
- The source-sync/build attempt was blocked by execution approval; no running
  container was replaced, no LTE state or business volume was removed. Native
  image build and handset SMS/data end-to-end acceptance remain outstanding.
  / 同步构建被执行审批拦截，未替换线上容器、未删除 LTE 或业务卷；原生镜像构建和真机业务验收待完成。

- OpenBTS stdout now retains at most 16 MiB plus one backup, preserves prior startup evidence, and detects readiness only in the current child session. Bounded pipe waiting avoids inherited descriptors hiding process exits. / OpenBTS 输出新增 16 MiB 加一份轮转，重启保留证据，就绪仅取本次进程输出，并限制继承管道等待。

- API completeness is checked across 14 path templates and 23 operations in the router, Markdown, OpenAPI and Postman. Actual middleware errors, the eight-key config allowlist and SMS pagination/window assertions are covered. / 14 个路径模板、23 项操作已对齐四套契约，并覆盖中间件错误、8 键配置白名单及短信分页窗口断言。

- Validation passed: Windows Go test/vet; Linux full race/vet; persistent-state, vendor-prefetch, Ubuntu/Windows deployment contracts; Postman offline script and schema/route checks. Native image execution remains pending. / Windows 与 Linux 离线测试、race/vet、持久化及部署构建契约、Postman 脚本与接口契约均通过；新版原生镜像执行验收仍待完成。

## Verified deployment, 2026-09-08 15:42 HKT / 已验证部署

The operator explicitly authorized source sync, detailed commits/GitHub push,
image build and replacement of the GSM container. This completes the deployment
that was pending in the preceding audit. 用户明确授权源码同步、详细提交及推送、构建和
更换 GSM 容器；本节完成上一节待处理的部署，不改写历史审计事实。

- Runtime implementation revision: `b830c6fd1119`; image
  `gsm-system:2.1.0-b830c6fd1119`, validated alias `gsm-system:2.1.0`.
- Image ID: `sha256:e9187d08d0ba4c31ebfed2941770872a7df8501b0c8d54d6c09f58b50c2dc383`.
- Container: `gsmsystem-uhd4`, ID `db0ea753292c774ef817fd9a5578be5fdcedd1298cb8bf6bf36d00bbead09a2f`.
- Go binary/OCI revision agree; native `GSM_SMS_V1` marker and loopback-only SIP
  contact ACL are present. `test_image.sh`, `test_presets_image.sh`, and
  `test_sms_image.sh` all passed with isolated volumes and no RF/USB access.
  / 二进制、标签一致；原生短信标记与本机 ACL 已入镜像，三套隔离镜像测试全部通过。
- Both native database integrity checks passed before maintenance. Subscriber
  SQL-dump SHA-256 matched exactly before and after recreation; three subscriber
  rows and three number bindings remain, with `quick_check=ok`.
  / 升级前后签约库 SQL 摘要完全一致，三条签约与三条号码绑定保留，完整性检查通过。
- Known old normal/open welcome defaults now explain replying a 7–10 digit
  number to `101` and sending `info` to `411`. Existing custom/blank messages are
  still preserved by the migration. / 已知旧默认欢迎文案迁移生效，自定义及留空设置不覆盖。
- Management is healthy, Token remains blank/disabled, all five cell processes
  are stopped and RF is off. Bridge mode and existing external data volume remain;
  only the operator's LAN management TCP `8082` is published.
  / 管理容器健康、仍为免 Token；五个小区进程全部停止。保留 bridge 与业务卷，仅发布管理端口。
- The container's `eth0` NAT setup returned `changed=true`, then `false` on repeat.
  GPRS settings and upstream DNS were preserved. Existing SMS history now returns
  six deduplicated observations; unkeyed legacy text remains unknown.
  / NAT 恢复且幂等，GPRS 配置保留；历史接口恢复六条去重观察，旧无键正文仍保持未知。
- LTE container ID, image ID, stopped state and start/finish timestamps are
  unchanged. No LTE service, business volume or subscriber record was deleted.
- The obsolete `08f184f063c5` GSM image and old container are gone. All test
  containers/volumes were removed. Builder pruning reported **3.526 GB** reclaimed;
  final build cache is **0 B**. Only the active GSM image ID with two aliases,
  LTE image, Go/Ubuntu bases and the two business volumes remain.
  / 旧 GSM 镜像及旧容器、隔离测试资源已清理；构建缓存回收 3.526 GB，最终为零。
- Implementation CI passed: [GitHub Actions run 34199517420](https://github.com/addxemmm/gsm-system/actions/runs/34199517420).
  Documentation-only commits after this record do not change the running image
  implementation or `.release-revision`. / 后续纯文档提交不改变运行镜像或构建版本标记。

Handset reply/confirmation SMS, peer SMS, two-way voice and Internet access still
require a separate live acceptance run after the operator starts the cell.
用户启动小区后仍须完成号码回复确认、手机互发短信、双向语音和互联网的真机验收；
本次没有发送空口短信或重新开启射频。

## Caller identity, SMS enrichment and GPRS update, 2026-09-08 16:32 HKT / 通话、短信与分组数据更新

The operator confirmed working handset registration and SMS transmit/receive,
but reported zero caller numbers, missing SMS identities and failed Internet
access. This release separates proven implementation repairs from remaining
handset acceptance. 用户确认接入与短信收发正常，本次针对零号码来电、短信身份缺项
和上网失败修复；以下区分已验证实现与尚待真机完成的验收。

- Runtime revision `5eff87bfa9be`; image `gsm-system:2.1.0-5eff87bfa9be`,
  release alias `gsm-system:2.1.0`; image ID
  `sha256:c2cedf2f5a02015c90f7736d378f94adc590cb6d8def5e7c0b15c1e549e72dae`.
- Container `gsmsystem-uhd4`, ID
  `0a67149da510433baf800087b7371b7b2865ed6a65dcbfa406e721f3987e5949`.
  Management is healthy, cell/RF stopped, Token blank/disabled. The existing
  bridge, LAN-only TCP 8082 publication and external business volume remain.
  / 管理健康、小区与射频停止、令牌保持留空；保留 bridge、局域网管理端口及业务卷。
- Caller-ID root cause: realtime subscribers enter `phones`, which previously
  skipped caller identity initialization. The outbound path then copied an empty
  CDR into caller ID. Both entry paths now resolve the configured SIP peer to its
  valid bound number; unknown/invalid mappings preserve existing identity.
  / 实时签约的 phones 入口此前遗漏主叫初始化，出局又用空 CDR 覆盖号码；现已统一
  通过驱动解析的 peer 查绑定，异常或未知映射保留原身份。
- The production SMS endpoint returned 12 observations: all 12 sender numbers
  resolved from current bindings and all three peer-message receiver IMSIs
  resolved. Three messages to 101 correctly retain no receiver IMSI; six legacy
  unkeyed texts remain unknown. Per-field provenance explicitly distinguishes
  log observations, query-time bindings and unknown values; no bodies or device
  identifiers are copied into this release record. / 线上 12 条观察均已补齐发送号码，
  三条点对点短信补齐接收 IMSI；三条发往 101 的 IMSI 仍为空，六条旧无键正文仍未知。
  各字段标注日志、当前绑定或未知来源，本记录不复制短信正文与设备身份。
- GMM optional-IE parsing accepts modern one-octet C/D/E/F fields, rejects
  malformed TLVs and avoids MBMS fallthrough. The native builder ran 144 cases
  against the extracted production method. A pre-patch negative control failed
  the valid terminal TV case. This fixes a definite defect consistent with the
  observed AttachRequest errors, not proof that every handset failure had that
  cause. / 原生构建验证生产方法 144 项用例，旧方法负对照复现合法 TV 失败；补丁修复
  确定缺陷，但未捕获本次手机具体 IE，不将全部失败武断归因或声称已经上网。
- Fresh volumes default to GPRS enabled, two C0 packet-data channels and a
  validated upstream DNS; existing native configuration remains unchanged.
  Invalid initialization can recover after configuration correction, including
  container PID reuse. Production GPRS was verified enabled, with its existing
  LAN upstream DNS preserved. Container eth0 forwarding/NAT was reapplied and
  returned changed=true then false. / 新卷默认开启分组数据、两个 C0 信道及上游 DNS；
  旧配置保留，错误配置修正后初始化可恢复。线上 GPRS 已开启、原 LAN DNS 保留，
  容器转发/NAT 已恢复并验证幂等。
- Four server-side isolated suites passed: image/persistence/Asterisk/ODBC/CDR,
  preset/custom start modes, SMS history/identity/deduplication, and caller-ID
  Local-channel mapping/unknown/invalid cases. No RF, production volume or
  handset messages were used by these tests. Windows Go test/vet, Linux race/vet,
  persistence and deployment/build contracts and Postman checks also passed.
  / 四套服务器隔离镜像测试与跨平台离线测试通过，不使用生产卷或射频发短信。
- Before replacement there were zero active calls; DELETE /api/v1/cell stopped
  the cell successfully. Subscriber SQL-dump digests matched before and after,
  with three subscribers, three bindings and quick_check=ok. LTE ID, image,
  stopped state and timestamps were unchanged. / 无通话时通过停止 API 维护；升级前后
  签约摘要一致、三条签约及绑定完整，LTE 容器与状态未变。
- Removed obsolete b830c6fd1119 and intermediate c2a08f02f48d images; no rollback
  or isolated test containers/volumes remain. Builder pruning reclaimed 3.638 GB;
  final build cache is 0 B. Only the current GSM image ID/two aliases, LTE image,
  Go/Ubuntu bases, two business containers and two business volumes remain.
  / 旧版及中间镜像、测试资源已清理；构建缓存回收 3.638 GB 后为零，业务数据保留。
- Final implementation CI passed:
  [GitHub Actions run 34204429022](https://github.com/addxemmm/gsm-system/actions/runs/34204429022).
  Documentation-only commits after this entry do not change runtime revision.
  / 最终实现 CI 通过；后续纯文档提交不更改镜像实现版本。

Remaining live acceptance: start the cell through preset or custom POST /cell,
reattach the test handsets, call in both directions and inspect incoming numbers;
then verify PDP address, bidirectional TUN/NAT traffic, DNS and handset HTTP.
待真机验收：通过预设或自定义请求启动小区，手机重新接入后双向互拨检查来电号码；
再验证 PDP 地址、TUN/NAT 双向流量、DNS 与手机 HTTP。当前并未自动重新开启射频。

## Current-start SMS scope and project timezone, 2026-09-08 17:19 HKT / 本次启动短信范围与项目时区

Deployed at 17:19 HKT and verified healthy after cleanup at 17:23 HKT.
2026-09-08 17:19 部署，17:23 清理后确认管理容器健康。

- Runtime revision `9512d5e4d850`; image `gsm-system:2.1.0-9512d5e4d850`,
  alias `gsm-system:2.1.0`; image ID
  `sha256:812aff2ed12b545b803d02a819364611e52347ca1bb54322a913ca1317cf04aa`.
  Container `gsmsystem-uhd4`, ID
  `203e23b6503024faa20e7baaee7f683ea68476545443602eb8628197f6119867`.
- The reported 16 observations included 12 from earlier runs and four from the
  latest previous start. GET /sms now reads only log bytes observed within the
  current accepted cell-start window, not all persisted history. Stop freezes
  the window; the next start establishes a new one. Manager/container restart
  does not adopt a previous run. Rejected parameters or duplicate running starts
  preserve the existing window. / 原 16 条混入 12 条旧记录；现在按本次小区启动的
  日志边界读取，停止后冻结，下一次启动新建范围，管理程序重启不继承旧范围。
  参数拒绝及重复启动不破坏现有范围。
- Responses expose `scope=current_start`, `timezone` and a nullable `session`
  with ID, start/end times and state. Event deduplication and current-binding
  identity provenance remain. Single log rotation and partial lines are covered;
  an unverifiable boundary returns an empty, explicitly truncated result rather
  than mixing old logs. Persistent queue messages processed during this start
  may still appear: the scope is observation time, not original submission time.
  / 新增范围、时区及会话元数据，保留事件去重与绑定来源；支持单次轮转与半行处理，
  边界丢失明确报告，不混入旧日志。旧队列若本次被处理，属于本次观察，不等于新提交。
- Default project zone is `Asia/Shanghai`; startup `TZ` overrides YAML `timezone`.
  Container local time, native logging and API timestamps now agree, with explicit
  RFC3339 offsets in API results. CDR storage remains UTC and only display is
  converted. Invalid zones fail startup. Set `TZ` in the project-root `.env` and
  recreate the container; no host clock is changed. Production verified
  `2026-09-08T17:19:50+08:00` and `/etc/timezone=Asia/Shanghai`.
  / 默认东八区，启动环境变量优先于 YAML；日志与接口统一，接口携带时区偏移；
  CDR 保持 UTC 存储。根目录 .env 修改 TZ 后重建容器生效，不修改宿主机时钟。
- Windows full Go test/vet and Linux race/vet passed, together with timezone,
  persistence, deployment/build contracts, OpenAPI and Postman checks. All four
  server-side image suites passed: general persistence/Asterisk/ODBC/CDR/timezone,
  preset/custom start, current-start SMS/identity semantics, and caller-ID.
  SMS lifecycle tests used inert native-process fixtures without RF, USB access,
  network access or production volumes. / 跨平台测试、契约和四套镜像测试通过；短信
  生命周期测试使用无射频的惰性进程样本，不访问 USB、网络或生产卷。
- Implementation CI passed:
  [GitHub Actions run 34208517698](https://github.com/addxemmm/gsm-system/actions/runs/34208517698).
  OpenAPI, bilingual API/deployment docs and Postman JSON were updated together.
  / 实现 CI 通过，OpenAPI、中英双语文档与 Postman 同步更新。
- Zero active calls before replacement; DELETE /cell stopped remaining services.
  Subscriber SQL-dump and SMS-log digests matched before/after deployment;
  quick_check=ok, three subscribers and three number bindings were retained.
  API Token remains blank/disabled. Bridge networking and LAN-only TCP 8082
  remain; eth0 forwarding/NAT returned changed=true then false on repeat.
  / 停机前无通话；升级前后签约与原始短信日志摘要一致，三条签约及号码绑定保留。
  Token 留空、bridge 与局域网管理端口保持不变，NAT 幂等验证通过。
- Production GET /sms returned count=0, scope=current_start, session=null and
  timezone=Asia/Shanghai because no cell has been started in the new manager.
  All five native cell processes are stopped and RF remains off. This is expected,
  not loss of the persisted historical log. / 新容器尚未启动小区，短信为空、session
  为 null 是预期结果；原始历史日志未删除，小区及射频保持停止。
- Removed obsolete `5eff87bfa9be` GSM image; the replaced container and all isolated
  test resources are gone. Builder pruning reclaimed 3.528 GB; final cache is 0 B.
  Only the current GSM image ID/two tags, LTE image, Go/Ubuntu bases, two business
  containers and two business volumes remain. LTE ID, image, stopped state and
  timestamps were unchanged. / 旧 GSM 镜像、旧容器及测试资源已清理，构建缓存回收
  3.528 GB 后为零；LTE 与两个业务卷保留。

The earlier UHD receive-timeout/degraded fault is not repaired by this change;
handset caller-ID and packet-data Internet still need separate live acceptance.
此前 UHD 接收超时导致 degraded 的问题不属于本次已修复范围，真机来电显示及上网
仍需独立验收。Documentation-only commits after this record do not change the
runtime implementation or `.release-revision` / 后续纯文档提交不改变运行实现版本。

## Chinese SMS, bounded RX recovery and diagnostic calls, 2026-09-08 18:42 HKT / 中文短信、有限接收恢复与测试通话

Implementation commits / 实现提交:
- `d901423be38e`: fixed runtime tag, gated cleanup, bounded UHD receive-timeout
  handling and restored 2600/2602 routes. / 固定运行标签、验收后清理、有限接收超时处理与测试号码路由。
- `8466188416a7`: native UCS-2 decoding and lossless observation tests.
  / 原生 UCS-2 解码与无损短信观察测试。

### Delivered and validated / 已交付与验收

- Image is now exactly `gsm-system:2.1`, with no alternate GSM tags. Application
  semantic version remains `2.1.0`; OCI and binary revision identify the source
  without a runtime tag suffix. Image ID:
  `sha256:0ecf8f12741072a55ff24f01322412ee5690e52814b4af1ae5b866548fc9a0aa`.
  Container `gsmsystem-uhd4`, ID:
  `7f7da87c176ec8429a1d83856cd01d07159a2781a9355cdf9006f56233b02b55`.
  / 运行镜像仅保留 2.1 标签，源码追踪使用元数据，不擅自升级版本或增加尾号。
- Chinese `text:null` originated in native `TLUserData::decode`, which rejected
  DCS 0x08 before Go parsed the observation. Patch 0006 decodes strict BMP UCS-2
  (0x08 and 0x18..0x1b), checks lengths/UDH and preserves forwarding bytes.
  Unsupported encodings, malformed data and surrogate code units stay unknown;
  multipart reassembly and API outbound Unicode are not added. Existing missing
  historical text cannot be reconstructed from an empty observation.
  / 根因是原生解码器拒绝中文编码而非 Go JSON；新增严格 BMP UCS-2 解码与边界校验，
  不改转发字节。不支持的编码、错误报文、代理码元保持未知；不新增分段重组或 API
  中文发送，旧空正文记录不会凭空恢复。详见 [SMS-UNICODE.md](SMS-UNICODE.md)。
- Old transceiver exited on the first 100 ms receive timeout, followed by an
  OpenBTS clock timeout. Patch 0005 retries a transient timeout for at most ten
  timeout events or one second and requires recovered sample continuity.
  Persistent faults, timestamp gaps and malformed metadata remain fatal; no
  sample fabrication, automatic RF restart or unlimited retry is introduced.
  / 旧版单次 100 ms 超时即退出；现在有限重试且验证样本连续性，持续故障或时间戳
  缺口仍停止，不伪造采样或自动重启射频。详见 [UHD-RX-RECOVERY.md](UHD-RX-RECOVERY.md)。
- Restored exact 2600 Echo and 2602 Milliwatt routes via phones/default/from-openBTS.
  Reserved-number binding validation and Postman/OpenAPI documentation are in
  sync. Six actual Local-channel calls reached the correct answered applications;
  no broad legacy demo/shell/recording routes were re-enabled.
  / 恢复两个测试号码，六条 Local 通道路由均实际接通对应应用；保留号码校验与文档
  同步，不恢复旧演示中的 shell/录音功能。详见 [VOICE-DIAGNOSTICS.md](VOICE-DIAGNOSTICS.md)。
- Native build passed UCS-2 decode/forwarding regressions and original-source
  negative control, 32 UHD RX checks and its original immediate-exit control,
  and 144 GMM optional-IE cases. Linux Go race/vet and all four final image suites
  passed: caller identity/diagnostics, general persistence/Asterisk/ODBC/CDR/TZ,
  current-start SMS, and presets/custom starts. Tests used isolated resources,
  no production volume or RF access.
  / 原生解码、UHD 32 项、GMM 144 项、旧代码负向对照、Go race/vet 与四套最终镜像
  验收全部通过；镜像测试使用隔离资源，不接入生产卷或射频。
- Implementation CI passed:
  [GitHub Actions run 34216249308](https://github.com/addxemmm/gsm-system/actions/runs/34216249308).
  / 最终实现提交的 GitHub CI 已通过。

### Production deployment and cleanup / 生产部署与清理

- Before replacement the old runtime was degraded with OpenBTS/transceiver absent,
  supporting services alive and zero calls. DELETE /cell stopped remaining services.
  No subscriber binding conflicted with the restored reserved service numbers.
  / 替换前旧版再次 degraded，无活动通话，停止残留服务后部署；测试保留号无绑定冲突。
- At 18:42 HKT the new container was running/healthy, direct binary probe passed,
  binary revision was `8466188416a7`, and all five native cell processes were
  stopped. RF was not automatically restarted. GET /sms correctly returned an
  empty current-start scope with session=null; historical files remain intact.
  / 18:42 管理容器健康、版本匹配，小区五进程全部停止且未自动开射频；新管理进程
  尚未启动小区，因此本次短信范围为空，历史日志保留。
- SQL-dump and raw SMS-log digests matched immediately before/after deployment;
  quick_check=ok, three subscribers and three bindings retained. Token remains
  blank/disabled; timezone is Asia/Shanghai and container time carried +08:00.
  Bridge networking publishes only LAN-bound TCP 8082. PUT /network eth0 returned
  changed=true then changed=false, confirming idempotent forwarding/NAT setup.
  / 部署前后 SQL 与短信日志摘要一致，三条签约及绑定保留；免 Token、东八区、bridge
  和局域网管理端口不变，出口 NAT 幂等验证通过。
- The deploy script verified revision, image identity and health before deleting
  superseded GSM images, the old runtime and unused builder cache. Only the
  current GSM tag, unchanged LTE image and required Go/Ubuntu build bases remain;
  build cache is 0 B. Two business containers and two external business volumes
  remain, no isolated test containers or volumes. LTE container/image IDs, stopped
  state and start/finish timestamps were unchanged. No volumes or networks were
  pruned; database, raw SMS logs and unrelated files were not treated as cache.
  / 清理在版本和健康验收通过后执行；仅保留当前 GSM 标签、原 LTE 及 Go/Ubuntu
  构建基础镜像，构建缓存为零，两个业务容器和两个数据卷保留。LTE 身份、状态与时间戳
  均未变化，未清理数据卷、网络、数据库或原始短信日志。

### Not yet proven on handsets / 尚未真机证明的部分

Peer/API SMS delay, difficult peer calls, physical USB/radio stability and GPRS
Internet are not declared fixed by these isolated checks. Logs also showed TCH
release/reassignment conflicts, late paging and peer-call SIP 502/cause 27.
The queue's 15-second acknowledgement wait plus 60-second retry delay can amplify
failed deliveries, but the existing log does not establish that path for each
reported message. Radio allocation and queue timers were not blindly shortened.

手机互发/API 下发延迟、互拨困难、物理 USB/无线稳定性及 GPRS 上网不等于已被这些隔离
测试证明修好。日志另有信道释放/重分配冲突、迟到寻呼和 SIP 502/cause 27；确认等待
与重试可能放大投递失败的延迟，但还缺少逐条关联证据，本次未盲目缩短定时器。

Next live acceptance: start the cell through the existing preset or custom API,
reattach the test phones, verify 2600 echo and 2602 tone first, then compare short
Chinese SMS in both directions with API observations. Time a distinct ASCII API
message and peer calls in both directions, recording test times rather than
publishing subscriber identifiers or message contents. Correlate new errors with
radio, paging and SIP/ACK logs before further tuning.

下一步真机验收：用预设或自定义 API 启动小区，手机重新接入；先测 2600 回声与 2602
测试音，再测双向短中文短信和记录一致性、API ASCII 下发时延及双向互拨。按测试时刻
关联无线/寻呼/SIP 确认日志，不公开用户身份和实际短信正文，再决定进一步参数调整。

Subsequent documentation-only commits do not change the deployed implementation
revision or `.release-revision`. / 后续纯文档提交不改变已部署的实现版本与发布标记。
