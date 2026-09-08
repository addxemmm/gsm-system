# Release 2.1 acceptance record / 2.1 发布验收记录

Version / 版本：`2.1.0`

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
