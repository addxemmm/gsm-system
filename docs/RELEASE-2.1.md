# Release 2.1 / 2.1 发布说明

Version / 版本：`2.1.0`

Image tags / 镜像标签：`gsm-system:2.1.0-<12sha>` (immutable / 不可变),
`gsm-system:2.1.0` (validated moving release tag / 验证后移动标签)

## Scope / 范围

- The project-owned HTTP and process-management plane is fully Go.
  自研 HTTP 与进程管理面全部重构为 Go。
- The public surface is only `/api/v1`; ad-hoc root routes and replaced methods
  are removed. 公开接口仅 `/api/v1`，移除旧根路径及已替换方法。
- Subscriber number binding is a resource (`PUT`/`DELETE`), connection state is
  separated from persistent subscribers, SMS returns `202 submitted`, call
  status uses active Asterisk channels, and history uses the real CDR CSV.
  号码绑定资源化；连接与持久签约分离；短信返回已提交；通话状态与历史来自 Asterisk。
- Network configuration is `GET` + idempotent `PUT` and reports
  `persisted:false`. 网络接口支持查询与幂等应用，并明确为运行时状态。
- One production Dockerfile and one Compose file replace the former split
  build definitions. 仅保留一份生产 Dockerfile 与 Compose。
- OpenAPI and a default-read-only Postman suite cover the 2.1 contract.
  OpenAPI 与默认只读 Postman 集覆盖 2.1 契约。

## Compatibility / 兼容性

This is an intentional breaking API release. Root endpoints return `404`;
`POST /api/v1/subscribers` and `POST /api/v1/network` return `405`. Clients must
migrate to the routes in [`API.md`](API.md). 这是有意的不兼容升级；客户端必须迁移。

The radio stack remains OpenBTS/Asterisk native C/C++. Python/Mako used by the
upstream UHD build is confined to a builder stage and is not a project-owned
runtime management service. 射频栈仍为原生 C/C++；UHD 上游构建用 Python/Mako 仅存在于
构建阶段，不属于运行时管理面。

## Data and operations / 数据与运维

- Existing `docker_gsm-data` is an external, operator-owned volume.
- Compose project: `gsm-system-live`; service `gsm-system`; container
  `gsmsystem-uhd4`, leaving stopped rollback container `gsmsystem` available.
- `/data/state` preserves native databases; `/data/log/asterisk/cdr-csv/Master.csv`
  preserves CDR history.
- Go syslog collector uses `/dev/log`, 16 MiB rotation, and one backup.

现有 `docker_gsm-data` 外部卷由运维管理，不随 Compose 删除；数据库与 CDR 均持久化；
普通发布与回滚流程绝不删除数据卷。

## Acceptance status / 验收状态

| Check / 检查 | Status / 状态 |
|---|---|
| source/API/docs refactor / 源码与文档重构 | repository candidate / 仓库候选态 |
| local `go test ./...` and `go vet ./...` | run before handoff / 交付前执行 |
| server image build and isolated smoke test | resumed after connectivity recovery; evidence pending / 网络恢复后继续，证据待完成 |
| live RF, handset attach, SMS, two-way voice | pending SDR-host acceptance / 待服务器验收 |

No 2.1 production deployment or handset result is asserted by this release note.
本文不宣称 2.1 已生产部署或已通过真机验收。

## Upgrade / 升级

Follow [`MIGRATION.md`](MIGRATION.md) and [`DEPLOY.md`](DEPLOY.md). Back up the
four native databases and CDR data before replacing a pre-persistence container;
retain the previous immutable image and stopped container until RF acceptance.
升级前按迁移与部署文档备份数据库/CDR，完成射频验收前保留旧镜像与停止态容器。
