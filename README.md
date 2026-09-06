# gsm-system（Go + OpenBTS）

![CI](https://github.com/addxemmm/gsm-system/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/badge/license-MIT-green)
![Go](https://img.shields.io/badge/go-1.22-blue)
![OpenBTS](https://img.shields.io/badge/OpenBTS-5.0-orange)

无状态、无Go层数据库的 GSM 自建基站工具：一个 Go 二进制暴露 HTTP API，编排容器内的 `OpenBTS` / `transceiver` / `sipauthserve` / `smqueue` / `asterisk`（USRP B210）。前端只调 API。

Stateless, Go-layer database-free GSM self-hosted base station toolkit: a single Go binary exposing an HTTP API that orchestrates `OpenBTS`/`transceiver`/`sipauthserve`/`smqueue`/`asterisk` (USRP B210) inside one container. No Go-layer DB, no frontend; API only. Docs are bilingual 中英双语 ([`docs/`](docs)); start with [`docs/QUICKSTART.md`](docs/QUICKSTART.md).

> 工作流：开发机上改代码，SDR 服务器上构建运行（本文示例 `vm-sdr`，按你的实际地址替换）。
>
> Workflow: edit code on the dev machine, build and run on the SDR server (example `vm-sdr`; replace with yours).

## 功能 Features

- 标准 REST：`/api/v1`（正确状态码 + `{"code","message","data","request_id"}` 包络 + OpenAPI，见 [`docs/API.md`](docs/API.md)）
  Standard REST: `/api/v1` (correct status codes + envelope + OpenAPI)
- 10 个根路径工具接口已冻结（逐字兼容，见 [`docs/API_LEGACY.md`](docs/API_LEGACY.md)）
  10 frozen root-path endpoints (verbatim compat)
- 新增 `GET /api/v1/ue|/sms|/subscribers|/config|/profile|/health`；`/api/v1/cell` 空 `{}` 复用上次配置（见 [`docs/RULES.md`](docs/RULES.md)）
  New explicit endpoints; empty `{}` reuses saved profile
- 签约显式化：SIM 外部写好，`POST /api/v1/subscribers` 显式设号码，无隐式行为（见 [`docs/SIM.md`](docs/SIM.md)）
  Explicit subscribers: SIMs provisioned externally, no implicit writes
- SDR：USRP B210（见 [`docs/SDR.md`](docs/SDR.md)）；短信/语音经 smqueue/Asterisk，中文短信不支持已文档化
  SDR: USRP B210; SMS/voice via smqueue/Asterisk; Chinese SMS unsupported (documented)

## 仓库布局 Repository Layout

```text
cmd/server            Go 入口 entrypoint
internal/api          v1 标准接口 + 旧版冻结 + 中间件（鉴权/审计/request-id）
internal/gsm          OpenBTS 启停 + 参数校验 + 预设 + sqlite/订阅管理
internal/parser       smqueue 短信 + sgsn/tmsis 解析（正则，无下标魔数）
internal/sdr          B210 探测
internal/sysop        无 shell 注入的进程管理（Z排除，失败路径回收）
internal/config       env+yaml 配置
compat/               UHD4 垫片 (msg) + 上游兼容补丁
firmware/uhd/         克隆板 FPGA + FX3 固件 (SHA256 锁定)
configs/              app.yaml.example + 干净 DB 种子 (seeds/)
deploy/docker/        Dockerfile.uhd4（现行，22.04+UHD4.1）/ Dockerfile（Xenial 备用）
                      + compose + entrypoint
postman/              全接口 Postman 集 (legacy+v1, 26 请求)
docs/                 QUICKSTART / API / API_LEGACY / RULES / SIM / SDR / MIGRATION / DEPLOY
scripts/              deploy_from_windows.ps1（git-archive 同步）+ prefetch_vendor.sh（服务端源码预取）
gsmsystem/            旧 Python 实现只读存档 v1 (Flask run.py + OpenBTS/asterisk dumps)
gsmsystem_v1.3/       旧手动启动脚本只读存档 (含DB重置 stop.sh)
docs/legacy/          旧中文手册存档 (520行 README.v1)
```

## 快速开始（服务器端） Quick Start (Server Side)

旧版容器首次升级须先按 [`docs/DEPLOY.md`](docs/DEPLOY.md) 备份并迁移四份数据库，
不要直接重建；本次审计及验收边界见 [`docs/AUDIT-2026-09.md`](docs/AUDIT-2026-09.md)。
For an existing pre-persistence container, back up and migrate the four native
databases before recreating it. See the deployment guide and audit report above.

```bash
cd ~/gsm-system
docker volume create "${GSM_DATA_VOLUME:-docker_gsm-data}"  # new installations / 新安装创建卷
docker compose -p gsm-system-live -f deploy/docker/docker-compose.uhd4.yml up -d --build
curl -s http://127.0.0.1:8082/api/v1/health; echo
```

启动小区示例（首启显式参数；之后 `{}` 复用）：
Example cell start (explicit first; `{}` reuses afterwards):

```bash
curl -X POST http://127.0.0.1:8082/api/v1/cell -H 'Content-Type: application/json' \
  -d '{"arfcns":"1","c0":"540","band":"1800","mcc":"001","mnc":"01","lac":"4420","ci":"41240","short_name":"test","network":"eth0"}'
```

完整入网→短信→互拨见 [`docs/QUICKSTART.md`](docs/QUICKSTART.md)（小区已点亮；入网待真机）。
Full attach→SMS→voice flow: [`docs/QUICKSTART.md`](docs/QUICKSTART.md) (cell verified live; UE attach pending handsets).

本地验证 Local checks (e.g. Windows PowerShell):

```powershell
go test ./...
go vet ./...
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o bin/gsm-system-linux-amd64 ./cmd/server
Remove-Item Env:\GOOS; Remove-Item Env:\GOARCH
```

旧版逐字接口见 [`docs/API_LEGACY.md`](docs/API_LEGACY.md)；v1↔旧版对照见 [`docs/API.md`](docs/API.md)。
Legacy verbatim routes: [`docs/API_LEGACY.md`](docs/API_LEGACY.md); map: [`docs/API.md`](docs/API.md).
