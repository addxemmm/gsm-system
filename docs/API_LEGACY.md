# Retired legacy API / 已退役旧接口

> Historical migration reference only. Release 2.1 does not register any root
> management endpoint; every public endpoint is under `/api/v1`. The old paths
> return the standard `40401` envelope and must not be restored.
> 本文仅供迁移查阅。2.1 不注册任何根路径管理接口，旧路径返回标准 `40401`，不得恢复。

The pre-2.1 API used action-style root routes, HTTP-200-only signaling, and a
`status/message_id/message` response shape. Those semantics are not part of the
2.1 compatibility contract. 旧版“恒 HTTP 200 + message_id”语义已退出兼容范围。

## Migration map / 迁移映射

| Pre-2.1 route / 旧路由 | Release 2.1 replacement / 2.1 替代接口 |
|---|---|
| `POST /start` | `POST /api/v1/cell` |
| `POST /stop` | `DELETE /api/v1/cell` |
| `POST /config` preset action | no replacement; send explicit cell parameters / 无预设替代，显式提交小区参数 |
| `POST /getconfig` | `GET /api/v1/config` |
| `POST /allconfig` | `PATCH /api/v1/config` with `{"values":{"KEY":"VALUE"}}` |
| `POST /iptables` | `GET /api/v1/network?iface=IFACE`, `PUT /api/v1/network` |
| `POST /smsinfo` | `GET /api/v1/sms` |
| `POST /ueinfo` | `GET /api/v1/connections` |
| `POST /setphonenumber` | `PUT` or `DELETE /api/v1/subscribers/{imsi}/number` |
| `POST /sendsms` | `POST /api/v1/sms` (HTTP `202` means submitted) |
| `GET /healthz` | `GET /api/v1/health` |
| `GET /status` | `GET /api/v1/cell` |
| `GET /profile` | `GET /api/v1/profile` |

`POST /api/v1/subscribers` and `POST /api/v1/network` were transitional v1
methods and are also removed; both return `40501` with `Allow` listing the
supported methods. Do not retry them against the old root routes. 过渡期 v1 POST
同样已移除，客户端必须更新方法与资源模型。

## Behavior changes / 语义变化

- Every response uses `code/message/data/request_id` and a meaningful HTTP
  status. `code=0` includes successful HTTP `202`.
- `/connections` is a volatile observation, while `/subscribers` is a persistent
  registry; “no connection” is not “subscriber offline”.
- Number binding is transactional and unique; optional TMSI projection is
  reported separately.
- SMS input is explicitly validated, and successful submission never claims
  delivery.
- Network configuration uses an idempotent `PUT` and reports
  `persisted:false`.
- Root-path response wording and legacy numeric `message_id` values are not
  preserved.

完整 2.1 契约见 [`API.md`](API.md)、[`api/openapi.yaml`](api/openapi.yaml) 与
[`MIGRATION.md`](MIGRATION.md)。A snapshot of older prose remains under
[`legacy/`](legacy/) for archaeology only; it is not an executable contract.
