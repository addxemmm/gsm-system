# gsm-system 2.1

![CI](https://github.com/addxemmm/gsm-system/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/badge/license-MIT-green)
![Go](https://img.shields.io/badge/go-1.22%2B-blue)
![OpenBTS](https://img.shields.io/badge/OpenBTS-5.0-orange)

`gsm-system` 是 USRP B210 GSM 实验系统的 Go 管理面，通过单一 `/api/v1`
REST API 编排 OpenBTS 5.0、transceiver、sipauthserve、smqueue 和 Asterisk。
2.1 只保留规范化 API，移除旧根路径手工接口与自研 Python 管理代码。

`gsm-system` is a Go control plane for a USRP B210 GSM lab system. It
orchestrates OpenBTS 5.0, the transceiver, sipauthserve, smqueue, and Asterisk
through one versioned REST API. Release 2.1 keeps only the standardized
`/api/v1` surface and removes the ad-hoc root endpoints and project-owned Python
management code.

> **2.1 status / 状态：** deployed and verified on 2026-09-08 16:32 HKT:
> image `gsm-system:2.1.0-5eff87bfa9be`, management healthy, cell stopped and RF off.
> The operator confirmed registration and SMS transmit/receive before this update.
> Caller-ID mapping, SMS current-binding provenance and modern GMM attach parsing
> are repaired; four isolated image suites passed, with three number bindings
> preserved. Handset caller display and packet-data Internet still need a live
> reattachment test; parser/Local-channel tests do not prove those outcomes.
>
> 2026-09-08 16:32 HKT 已部署上述镜像，管理容器健康，小区停止、射频关闭。
> 用户已确认接入及短信收发正常；本次修复主叫映射、短信当前绑定补全及现代 GMM
> 附着解析，四套隔离镜像测试通过，三条绑定保留。真机来电显示和互联网仍待重连验收，
> 不以解析器或 Local 通道测试代替。详见 [发布记录](docs/RELEASE-2.1.md)。

## Highlights / 主要特性

- Go standard-library HTTP control plane with a uniform
  `code/message/data/request_id` envelope and `X-Request-ID`.
  Go 标准库 HTTP 管理面，统一响应包络与请求 ID。
- Explicit resources for cell, user-managed presets, configuration,
  connections, subscribers, number binding, SMS, calls/history, network state,
  profile, and health. 小区、用户管理预设、配置、连接、签约、号码绑定、
  短信、通话/历史、网络、存档与健康资源。
- Idempotent network configuration and explicit number bind/unbind semantics.
  出口 NAT 幂等配置，号码绑定/解绑语义明确。
- OpenAPI 3.0 and a Postman collection grouped by read-only queries, cell
  start, and manually sent mutations. 提供 OpenAPI 3.0，Postman 集合按
  只读查询、小区启动与手动发送写操作分组。
- Persistent native OpenBTS/Asterisk databases and Go-native syslog capture.
  持久化 OpenBTS/Asterisk 原生数据库，Go 原生 syslog 采集。

The runtime image contains no Python management runtime. UHD's upstream build
system still uses Python/Mako in a **builder stage**; OpenBTS and Asterisk remain
their native C/C++ implementations. 运行时不包含 Python 管理程序；UHD 上游构建阶段
仍需 Python/Mako，OpenBTS/Asterisk 仍为原生 C/C++。

## API overview / API 概览

Base URL: `http://HOST:8082/api/v1`

| Resource / 资源 | Methods / 方法 | Purpose / 用途 |
|---|---|---|
| `/cell` | `GET POST DELETE` | status, start, idempotent stop / 状态、启动、幂等停止 |
| `/presets` | `GET POST` | list/create user presets / 列出/创建用户预设 |
| `/presets/{id}` | `GET PUT DELETE` | read/replace/delete a preset / 读取/替换/删除预设 |
| `/config` | `GET PATCH` | OpenBTS settings / OpenBTS 配置 |
| `/profile`, `/health` | `GET` | saved profile and health / 存档与健康 |
| `/connections` | `GET` | volatile connection observations, not online status / 易失连接观察，非在线判定 |
| `/subscribers` | `GET` | subscriber list / 签约列表 |
| `/subscribers/{imsi}` | `GET` | subscriber detail / 签约详情 |
| `/subscribers/{imsi}/number` | `PUT DELETE` | bind or unbind number / 绑定或解绑号码 |
| `/sms` | `GET POST` | list and submit SMS / 列表与提交短信 |
| `/calls`, `/calls/history` | `GET` | active channels and CDR history / 活动通道与 CDR |
| `/network` | `GET PUT` | inspect/apply uplink NAT / 查询/幂等配置 NAT |

Root paths such as `/start`, `/stop`, `/ueinfo`, `/sendsms`, and `/iptables`
are retired and return `404`. Removed methods such as `POST /api/v1/subscribers`
and `POST /api/v1/network` return `405`. 旧根路径已下线；已替换的方法不再兼容。

Preset storage starts with five editable defaults (`"0"` through `"4"`) and
persists in `/data/presets.json`. `POST /cell` accepts complete explicit fields,
`{"preset_id":"0"}`, or `{}` to reuse the last profile. Preset CRUD never
changes a running cell and never autostarts one. 预设库初始包含 `"0"` 至 `"4"`
五套可编辑默认配置；预设 CRUD 不影响运行中小区，也不会自启动。

Re-import the current collection, then use your configured environment or the
**gsm-system 2.1 example / 环境示例** template so `baseUrl` targets the
intended host; `token` remains optional. Use
**Read-only / 查询接口** for GET-only inspection, choose exactly one preset
or custom request under **Cell start / 小区启动（二选一）**, and send requests
under **Mutations / 写操作（手动发送）** individually. Do not run the full
collection: start, delete, and SMS requests have real effects and no extra
enable switch. 必须重新导入新集合；环境可沿用已配置的环境或参照示例，
旧环境里的两个开关可删除，留着也不再生效。`verify_factory_defaults`
仅是独立的响应断言选项，不影响发送。

Full contract / 完整契约：

- [API guide / API 指南](docs/API.md)
- [OpenAPI 3.0](docs/api/openapi.yaml)
- [Postman collection](postman/gsm-system.postman_collection.json)
- [Postman example environment](postman/gsm-system.postman_environment.example.json)

## Radio constraints / 射频参数约束

- `arfcns` is exactly `1` in 2.1 / 2.1 仅支持单载频。
- GSM 900: `c0` is `0..124` or `975..1023`; DCS 1800: `512..885`.
- `lac`: `1..65279` (software-compatible range); `ci`: `0..65535`.
- SMS submission supports only the safe ASCII intersection of the GSM default
  alphabet, up to 159 bytes. No UCS-2/Chinese, extension-table characters,
  quotes, or backticks. The API reports **submitted**, never delivered.
  短信仅支持 GSM 默认基本表的安全 ASCII 交集，最多 159 字节；不支持
  UCS-2/中文、扩展表字符、引号或反引号。成功只表示已提交，不表示已送达。

## Repository / 仓库结构

```text
cmd/server/             Go entry point / Go 入口
internal/api/           HTTP routing, middleware, handlers / HTTP 路由与处理
internal/gsm/           OpenBTS orchestration and validation / OpenBTS 编排与校验
internal/subscriber/    subscriber and number binding / 签约与号码绑定
internal/logsink/       native syslog collector / Go 原生日志采集
internal/contract/      API/docs/Postman drift tests / 契约防漂移测试
configs/                non-secret examples and seeds / 无密钥示例与种子
deploy/docker/          the single Dockerfile and Compose file / 唯一构建配置
postman/                v2.1 collection and example environment
docs/                   bilingual operations and API documentation
```

## Local checks / 本地检查

```powershell
go test ./...
go vet ./...
$env:GOOS="linux"; $env:GOARCH="amd64"
go build -o bin/gsm-system-linux-amd64 ./cmd/server
Remove-Item Env:\GOOS; Remove-Item Env:\GOARCH
```

## Build and deploy / 构建与部署

Build and RF operations belong on `HOST`:

```bash
cd ~/gsm-system
docker volume inspect docker_gsm-data >/dev/null || docker volume create docker_gsm-data
./scripts/prefetch_vendor.sh
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live
curl -fsS http://127.0.0.1:8082/api/v1/health
```

Copy [`.env.example`](.env.example) to the repository-root `.env`. Leave
`GSM_API_TOKEN=` blank to disable Bearer authentication, or set a non-empty
value to enable it; `.env` is ignored by Git. 修改令牌后只需重建容器，无需重构镜像：

```bash
docker compose --env-file .env -p gsm-system-live \
  -f deploy/docker/docker-compose.yml \
  up -d --no-build --force-recreate gsm-system
```

将 [`.env.example`](.env.example) 复制为项目根 `.env`：`GSM_API_TOKEN=`
留空即关闭 Bearer 鉴权，填写非空值即启用；`.env` 已被 Git 忽略。

The release script builds immutable `gsm-system:2.1.0-<12sha>`, validates the
container and HTTP endpoint, then moves `gsm-system:2.1.0` to the same image ID.
It never deletes the external `docker_gsm-data` volume. Read
[`docs/DEPLOY.md`](docs/DEPLOY.md) before migrating or cleaning Docker objects.
The Compose service is `gsm-system`; its container is `gsmsystem-uhd4`. The
server currently retains only the active GSM image ID and its two tags; old
images and stopped rollback containers were removed after acceptance.

发布脚本构建不可变 tag `gsm-system:2.1.0-<12sha>`，容器与 HTTP 验证通过后，
再将 `gsm-system:2.1.0` 指向同一镜像 ID。脚本不删除外部数据卷。服务器当前只保留
在用 GSM 镜像 ID 及两个标签，不保留旧镜像或停止的回滚容器。

## Documentation / 文档

- [Quick start / 快速开始](docs/QUICKSTART.md)
- [Deployment, cleanup, rollback / 部署、清理、回滚](docs/DEPLOY.md)
- [Operating rules / 运行规则](docs/RULES.md)
- [SIM and number binding / SIM 与号码绑定](docs/SIM.md)
- [SDR notes / SDR 说明](docs/SDR.md)
- [2.1 migration / 2.1 迁移](docs/MIGRATION.md)
- [2.1 release notes / 2.1 发布说明](docs/RELEASE-2.1.md)
- [Historical audit / 历史审计](docs/AUDIT-2026-09.md)

Use only where local spectrum rules and lab authorization permit. Keep a
hardware RF kill path and stop the cell before changing RF-sensitive settings.
仅在本地频谱法规与实验授权允许的环境使用，保留硬件断射频手段，并在修改
射频相关配置前停止小区。
