# Operating rules / 使用规则

These rules describe release 2.1. 本文定义 2.1 的运行边界。

## 1. Control plane and native state / 管理面与原生状态

The project-owned control plane is Go-only and has no account database or job
queue. OpenBTS and Asterisk retain their native SQLite state. 项目自研管理面全部使用
Go，不维护账号数据库或任务队列；OpenBTS/Asterisk 继续使用各自原生 SQLite 状态。

| State / 状态 | Location / 位置 | Persistence / 持久性 |
|---|---|---|
| last cell profile / 上次小区参数 | `/data/last_start.json` | persistent, mode `0600` |
| OpenBTS databases | `/data/state/OpenBTS/` | persistent external volume |
| Asterisk registry | `/data/state/asterisk/sqlite3.db` | persistent external volume |
| current TMSI table / 当前连接表 | `/var/run/TMSITable.db` | volatile / 易失 |
| API/native logs / 日志 | `/data/log/` | persistent with rotation / 持久并轮转 |
| Asterisk CDR | `/data/log/asterisk/cdr-csv/Master.csv` | persistent / 持久 |

Profile writes use a same-directory temporary file, `fsync`, atomic rename, and
`0600`. 配置存档采用同目录临时文件、`fsync`、原子替换与 `0600` 权限。

## 2. API-only operation / 仅使用版本化 API

All public routes are below `/api/v1`. Retired root routes are intentionally not
registered; do not recreate wrappers for `/start`, `/stop`, `/sendsms`,
`/setphonenumber`, or `/iptables`. 所有公开接口均位于 `/api/v1`；不得恢复旧根路径包装。

Every response uses:

```json
{"code":0,"message":"ok","data":{},"request_id":"REQUEST_ID"}
```

- `code=0` means the HTTP operation succeeded; always inspect the HTTP status.
- Send `Authorization: Bearer TOKEN` only when `GSM_API_TOKEN` is configured.
- JSON mutation bodies are one object. Unknown fields and malformed/trailing
  JSON are rejected.
- Use `X-Request-ID`/`request_id` when correlating `/data/log/system.log`.

The YAML loader also rejects unknown keys and multiple documents. Release 2.1
removes `default_*`, `smqueue_seed_path`, and `max_upload_bytes`; copied configs
must delete them. `max_history_bytes` bounds SMS/CDR reads (default 8 MiB,
allowed 64 KiB..64 MiB). YAML 配置严格拒绝未知项和多个文档；迁移时删除旧字段。

## 3. Cell lifecycle / 小区生命周期

- Release 2.1 supports exactly one ARFCN: `arfcns="1"`.
- GSM 900: `c0=0..124` or `975..1023`; DCS 1800: `512..885`.
- `lac=1..65279` is the software-compatible range; `ci=0..65535`.
- First start must submit all required parameters. Empty `{}` may reuse a valid
  saved profile; it is not a hidden preset.
- `DELETE /api/v1/cell` is idempotent. Shutdown sends TERM before KILL so
  Asterisk can flush CDR data.
- `ready` means the five managed services are alive; it does **not** prove RF,
  handset attach, SMS delivery, or voice quality.
- `state` is `stopped`, `transitioning`, `degraded`, or `running`.

首次启动必须显式提供完整参数；之后空对象只能复用有效存档。`ready` 仅表示五个托管
进程存活，不代表射频、入网、短信送达或语音质量通过。停止流程先 TERM 后 KILL。

## 4. Connections, subscribers, and numbers / 连接、签约与号码

`GET /connections` is a view of current/volatile attach state.
`GET /subscribers` is the persistent registry. They are not interchangeable.
号码更新 is scoped to one subscriber:

```text
GET    /api/v1/subscribers/{imsi}
PUT    /api/v1/subscribers/{imsi}/number
DELETE /api/v1/subscribers/{imsi}/number
```

A SIM must be provisioned externally and must exist in the native registries.
Binding updates the related native records transactionally; unbinding removes
number routing but does not erase or rewrite the SIM. Never use a production
IMSI for destructive tests. Use a dedicated test SIM/IMSI and retain a database
backup. SIM 必须在外部制卡；绑定/解绑不写 SIM。破坏性测试只用独立测试 IMSI 并先备份。

Do not bind `111`, `112`, or `911`. This isolated dialplan has no PSTN connection
and does not guarantee real emergency calling. 禁止绑定上述保留码；实验拨号计划不提供
真实 PSTN 或紧急呼叫保证。

## 5. SMS and calls / 短信与通话

SMS submission is intentionally narrower than “GSM-7 support”:

- safe ASCII/GSM-default-basic intersection only;
- at most 159 bytes (the upstream CLI appends a space);
- no UCS-2/Chinese, extension-table characters, single/double quotes, or
  backticks;
- `202 sms submitted` acknowledges upstream queue submission only, never
  handset delivery.

短信接口只支持安全 ASCII/GSM 默认基本表交集，最多 159 字节，不支持中文/UCS-2、
扩展表、单双引号或反引号。HTTP `202` 只代表已提交上游队列。

`GET /calls` reports active Asterisk channels. Its count is the number of
currently returned channels, not a lifetime call counter. `GET /calls/history`
parses the real Asterisk CSV CDR; it does not synthesize calls. `/calls` 的计数是
当前活动通道数；历史来自真实 CDR CSV。

## 6. Network configuration / 网络配置

- `GET /api/v1/network?iface=IFACE` inspects the managed MASQUERADE rule.
- `PUT /api/v1/network {"iface":"IFACE"}` applies it idempotently.
- A success payload reports `persisted:false`: the rule is runtime state and
  must be applied again after host firewall/network reset.
- `ipv4_forwarding` is a read-only `true|false|null` observation. The API does
  not change sysctl, and `rule_present` alone does not prove forwarding works.
- Interface names are passed as argv, validated, and never interpolated into a shell.

网络 PUT 幂等，不重复追加规则；`persisted:false` 表示主机防火墙重置后需要重新应用。

## 7. Logs / 日志

Go binds `/dev/log` only when it is a socket path it can safely own; it does not
overwrite an active socket or regular file. It routes messages into
`smqueue.log`, `openbts-syslog.log`, and `system.log`, each mode `0600`, rotated
at 16 MiB with one `.1` backup. OpenBTS startup stdout/readiness remains in
`openbts.log`. Go 只在安全时接管 `/dev/log`，不覆盖活动 socket 或普通文件；各日志
16 MiB 轮转并保留一份备份。

## 8. Change discipline / 变更纪律

- API changes must update `docs/API.md`, `docs/api/openapi.yaml`, Postman, and
  `internal/contract` tests together.
- `go test ./...` and `go vet ./...` are development-machine checks.
- Docker image build, image smoke test, cell start, and RF acceptance run only
  on the SDR host.
- The Postman environment defaults `enable_mutations=false`; set it to `true`
  only for an isolated test window, then restore it.
- Never commit real IMSIs, numbers, tokens, IPs, PCAPs, logs, or databases.

API、OpenAPI、Postman 与契约测试必须同步修改。Docker/RF 仅在服务器执行，仓库不得
提交真实 IMSI、号码、令牌、地址、抓包、日志或数据库。
