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
