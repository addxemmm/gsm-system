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
| numeric carrier presets/default fields | explicit cell profile only / 仅显式小区参数 | remove deprecated YAML keys and send all first-start fields / 删除弃用配置并显式首启 |

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
were tied to pre-2.1 preset/fallback behavior; the first `POST /api/v1/cell`
must now contain a complete valid profile. 2.1 严格拒绝未知配置项与多文档 YAML；旧字段
不会被静默忽略，首次启动必须显式提交完整参数。

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
validation does it tag the same image as `gsm-system:2.1.0`. Keep the previous
SHA tag for rollback. 脚本先构建 SHA 标签，验证通过后再更新发布标签；旧 SHA 必须保留。

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

## 7. Rollback / 回滚

Select the retained immutable tag and skip build:

```bash
GSM_IMAGE=gsm-system:2.1.0-OLD12SHA \
  ./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live --skip-build
```

If a data schema/native configuration change was made after upgrade, stop writes
and restore the matching verified database snapshots as one rollback set.
回滚镜像时如数据已变化，应暂停写入并将同批验证快照整体恢复，不能混用不同时间的数据库。

The former audit remains as historical context only: [`AUDIT-2026-09.md`](AUDIT-2026-09.md).
旧审计仅作历史上下文，不是 2.1 当前契约。
