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
