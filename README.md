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

> **2.1 status / 状态：** verified at 2026-09-08 18:42 HKT:
> image `gsm-system:2.1`, revision `8466188416a7`, management healthy, cell stopped
> and RF off. Native Chinese UCS-2 SMS observations, bounded transient UHD RX
> timeout recovery and built-in 2600/2602 voice routes are included. Four isolated
> image suites, native regression/negative-control tests and CI passed.
> Three bindings and raw SMS history were preserved; obsolete GSM images and
> containers were removed, build cache is 0 B. Phone-to-phone/API SMS latency,
> radio call reliability and packet-data Internet still require live acceptance.
>
> 2026-09-08 18:42 HKT 验证修复版健康，镜像固定为 `gsm-system:2.1`，小区及射频停止。
> 已包含原生中文短信解码、短暂 UHD 接收超时的有限重试及 2600/2602 语音路由；四套
> 镜像测试、原生回归/旧版负向对照及 CI 通过。三条绑定和原始短信日志完整，旧 GSM
> 镜像与容器已清理，构建缓存为零。短信延迟、空口互拨可靠性及上网仍待真机验收。
> 短信保持本次小区启动范围，默认东八区可由启动 `TZ` 自定义。
> 详见 [发布记录](docs/RELEASE-2.1.md)。

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
- API SMS submission (`POST /sms`) supports only the safe ASCII intersection of the GSM default
  alphabet, up to 159 bytes. No UCS-2/Chinese, extension-table characters,
  quotes, or backticks. The API reports **submitted**, never delivered.
  API 短信发送仅支持 GSM 默认基本表的安全 ASCII 交集，最多 159 字节；不支持
  UCS-2/中文、扩展表字符、引号或反引号。成功只表示已提交，不表示已送达。
- SMS observations (`GET /sms`) separately support decoded BMP UCS-2 Chinese
  from patched smqueue. They cover the current cell start, not all historical
  logs or confirmed deliveries. 接收记录可包含中文；范围是本次小区启动，
  不是完整历史或送达回执。详见 [短信解码](docs/SMS-UNICODE.md)。

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
# Read-only state-aware probe; uses the container's optional Token internally.
# 只读状态探针，内部遵循容器的可选令牌配置，不输出令牌。
docker exec gsmsystem-uhd4 /usr/local/bin/gsm-system --healthcheck
```

Copy [`.env.example`](.env.example) to the repository-root `.env`. Leave
`GSM_API_TOKEN=` blank to disable Bearer authentication, or set a non-empty
value to enable it; `.env` is ignored by Git. 修改令牌后只需重建容器，无需重构镜像：

```bash
# Stop the cell before changing .env; retain the deployed .release-revision.
# 先停止小区再修改 .env，保留与镜像匹配的发布标记。
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live --skip-build
```

将 [`.env.example`](.env.example) 复制为项目根 `.env`：`GSM_API_TOKEN=`
留空即关闭 Bearer 鉴权，填写非空值即启用；`.env` 已被 Git 忽略。

The release script builds only `gsm-system:2.1`, validates its OCI/binary source
revision before deployment, then validates the container, HTTP endpoint, and
state-aware health probe. After success it removes only obsolete GSM runtime
objects and unused build cache; it never deletes the external `docker_gsm-data`
volume or LTE containers/images. Read
[`docs/DEPLOY.md`](docs/DEPLOY.md) before migrating or cleaning Docker objects.
The Compose service is `gsm-system`; its container is `gsmsystem-uhd4`. The
server retains only the active `gsm-system:2.1` runtime image; source identity
is recorded by its OCI revision label rather than a revision-suffixed tag.

发布脚本仅构建 `gsm-system:2.1`，部署前核对 OCI/二进制 revision；容器、HTTP 与
状态探针通过后才清理旧 GSM 对象及无用构建缓存，不删除业务数据卷或 LTE 对象。

The container uses `restart: "no"`, not boot/exit autostart. After reboot, start
the existing container manually with `docker start gsmsystem-uhd4`; then inspect
state and start the cell explicitly through its API. 容器不开机自启，退出也不自动重启；
需要时手动启动管理容器，再按流程启动小区。详见 [启动策略](docs/DEPLOY.md)。

## Documentation / 文档

- [Quick start / 快速开始](docs/QUICKSTART.md)
- [Daily operations and troubleshooting / 日常操作与排障](docs/OPERATIONS.md)
- [Postman setup and test interpretation / Postman 配置与测试结果解读](postman/README.md)
- [Deployment, cleanup, current-version recovery / 部署、清理与当前版本恢复](docs/DEPLOY.md)
- [Operating rules / 运行规则](docs/RULES.md)
- [SIM and number binding / SIM 与号码绑定](docs/SIM.md)
- [SDR notes / SDR 说明](docs/SDR.md)
- [Bounded UHD timeout recovery / UHD 有限超时恢复](docs/UHD-RX-RECOVERY.md)
- [2600/2602 voice diagnostics / 语音诊断](docs/VOICE-DIAGNOSTICS.md)
- [Chinese SMS decoding / 中文短信解码](docs/SMS-UNICODE.md)
- [GPRS identity and NAT recovery / GPRS 身份交换与 NAT 恢复](docs/GPRS-RECOVERY.md)
- [2.1 migration / 2.1 迁移](docs/MIGRATION.md)
- [2.1 release notes / 2.1 发布说明](docs/RELEASE-2.1.md)
- [Historical audit / 历史审计](docs/AUDIT-2026-09.md)

Use only where local spectrum rules and lab authorization permit. Keep a
hardware RF kill path and stop the cell before changing RF-sensitive settings.
仅在本地频谱法规与实验授权允许的环境使用，保留硬件断射频手段，并在修改
射频相关配置前停止小区。
