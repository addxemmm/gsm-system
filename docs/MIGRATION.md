# Migration to 2.1 / 迁移到 2.1

## 1. What changed / 变更内容

| Before / 之前 | 2.1 | Required action / 操作 |
|---|---|---|
| project-owned Flask/Python management | one Go binary / 单一 Go 二进制 | remove old runtime and wrappers / 移除旧运行时与包装 |
| root `/start`, `/stop`, `/ueinfo`, etc. | `/api/v1` resources | update every client / 更新全部客户端 |
| mixed “UE/subscriber” semantics | `/connections` + `/subscribers` | choose volatile vs persistent resource / 区分连接与签约 |
| set-number action endpoint | `PUT/DELETE .../{imsi}/number` | model number as a resource / 将号码作为资源 |
| SMS success looked final | HTTP `202`, `sms submitted` | track delivery outside this acknowledgement / 不将提交当送达 |
| appended iptables rules | `GET/PUT /network`, idempotent | use PUT and reapply after host reset / 改用 PUT |
| multiple Docker definitions/tags | one Dockerfile/Compose; versioned tags | use release script / 使用发布脚本 |
| numeric carrier presets/default fields | user-managed `/api/v1/presets` / 用户管理预设 | remove deprecated YAML keys; explicitly create required presets / 删除弃用配置并显式创建预设 |

UHD still uses Python/Mako as an upstream **build-time** dependency. OpenBTS,
Asterisk, and UHD are not rewritten in Go. Only the project-owned control plane
is Go-only. UHD 上游构建依赖仍可能包含 Python/Mako；Go 重构范围是自研管理面。

## 2. API client migration / API 客户端迁移

| Removed / 已移除 | Replacement / 替代 |
|---|---|
| `POST /start` | `POST /api/v1/cell` |
| `POST /stop` | `DELETE /api/v1/cell` |
| `POST /ueinfo` | `GET /api/v1/connections` |
| `POST /smsinfo` | `GET /api/v1/sms` |
| `POST /sendsms` | `POST /api/v1/sms` |
| `POST /setphonenumber` | `PUT /api/v1/subscribers/{imsi}/number` |
| `POST /getconfig` | `GET /api/v1/config` |
| `POST /allconfig` | `PATCH /api/v1/config` |
| `POST /iptables` | `PUT /api/v1/network` |
| `GET /healthz`, `/status`, `/profile` | versioned `/api/v1/health`, `/cell`, `/profile` |
| legacy preset IDs/root preset routes | `GET/POST /api/v1/presets`, `GET/PUT/DELETE /api/v1/presets/{id}` |

Do not retry old methods on `405`; update the client. Root paths intentionally
return `404`. 旧方法收到 `405` 时不要降级重试；根路径返回 `404` 是预期行为。

`PATCH /api/v1/config` now updates a map atomically at the API boundary:

```json
{"values":{"GSM.Identity.ShortName":"LAB","Control.LUR.OpenRegistration":"REGEX"}}
```

The old `{name,value}` body is rejected. 旧单键请求体不再接受。

## 3. Configuration migration / 配置迁移

The 2.1 YAML loader rejects unknown keys and multiple YAML documents. Before
deployment, remove these retired fields from copied/custom configuration:

- `default_arfcns`, `default_c0`, `default_band`, `default_mcc`, `default_mnc`,
  `default_lac`, `default_ci`, `default_short_name`, `default_network`;
- `smqueue_seed_path`;
- `max_upload_bytes`.

Use `max_history_bytes` for bounded SMS/CDR log reads. Its default is `8388608`
(8 MiB) and accepted range is `65536..67108864`. The removed `default_*` fields
were tied to pre-2.1 preset/fallback behavior. They do not recreate old IDs.
Create every needed preset explicitly through `/api/v1/presets`, or submit a
complete valid profile on the first `POST /api/v1/cell`. A new data volume has
no operator/carrier preset seeds. 2.1 严格拒绝未知配置项与多文档 YAML；旧字段不会
恢复旧 ID，新数据卷不包含操作员/运营商种子；请显式创建新预设或完整提交首启参数。

User presets are stored in `/data/presets.json`; the independent last-start
profile remains `/data/last_start.json`. Existing saved profiles or manually
migrated presets must set required `network` to the **container** interface
`eth0`, not a host NIC such as `ens33`. Stop RF first and update the persisted
JSON atomically. 预设与上次启动存档分别持久化；迁移时将 `network` 改为容器内
`eth0`，而非宿主机网卡，并在停止 RF 后原子更新。

## 4. Back up native state / 备份原生状态

Before recreating an older container, stop the cell and pause management writes.
Use SQLite `.backup` for OpenBTS, sipauthserve, smqueue, and Asterisk; also copy
`Master.csv` if present. Verify each database with `PRAGMA quick_check`.
重建旧容器前先停止小区、暂停写入，使用 SQLite `.backup` 备份四个数据库并复制 CDR，
随后执行 `PRAGMA quick_check`。完整命令见 [`DEPLOY.md`](DEPLOY.md)。

Never delete `docker_gsm-data` during migration. 迁移期间严禁删除数据卷。

## 5. Deploy immutable image / 部署不可变镜像

```bash
cd ~/gsm-system
docker volume inspect docker_gsm-data >/dev/null
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live
```

The script builds `gsm-system:2.1.0-<12sha>`. Only after container/HTTP
validation does it tag the same image as `gsm-system:2.1.0`. After acceptance,
the operator chose to retain only the active image ID and its two tags; old
images, the stopped rollback container, and old host backup directories were
removed while the external business-data volume was kept. 验收后按当前策略只保留
在用镜像 ID 及两个标签；旧镜像、停止的回滚容器及主机旧备份目录已删除，外部业务
数据卷仍保留。

## 6. Validate in order / 按顺序验收

1. `go test ./...` and `go vet ./...` on the development checkout.
2. Server-side isolated image smoke test without RF.
3. `GET /api/v1/health`, `/cell`, `/profile`, `/network`.
4. Start one legal test cell and check `state`, `ready`, `sms_ready`,
   `voice_ready`.
5. Attach a dedicated test SIM; compare `/connections` and `/subscribers`.
6. Bind a test number; submit safe-ASCII SMS; verify reception separately.
7. Place a two-way call; compare active channels and `/calls/history` CDR.
8. Stop the cell and verify log/CDR/database persistence after recreation.

Steps 1–2 and the applicable non-RF portions of steps 3 and 8 passed on
2026-09-08. A local
Asterisk bridge fixture also verified the calls API and a real 18-column
`ANSWERED` CDR without SIP, handsets, or RF. Steps 4–7 and handset/RF portions
of step 8 remain pending. 2026-09-08 已完成构建、隔离镜像、管理面及非射频持久化验收；
本地 Asterisk 桥接验证不等于真机/SIP/RF 验收，步骤 4–7 仍待执行。

## 7. Current retention policy / 当前保留策略

There is no retained previous image or stopped rollback container. Future
recovery uses a deliberately selected source revision and a fresh deployment of
the current version. The external `docker_gsm-data` volume remains
operator-owned and must not be deleted. 当前没有保留旧镜像或停止的回滚容器；后续恢复
应明确选择源码版本并重新部署当前版本。外部 `docker_gsm-data` 卷仍由运维持有，不得删除。

The former audit remains as historical context only: [`AUDIT-2026-09.md`](AUDIT-2026-09.md).
旧审计仅作历史上下文，不是 2.1 当前契约。
