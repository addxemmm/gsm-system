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
