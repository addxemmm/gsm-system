# GSM-System API v1 标准接口 Standard API

> Legacy frozen routes 旧冻结路由见 [`API_LEGACY.md`](API_LEGACY.md).
> Machine-readable contract 机器契约：[`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.0).

- Base 基地址：`http://<server>:8082` (host `http://127.0.0.1:8082`)
- Envelope 包络：`{"code": int, "message": str, "data": obj|null, "request_id": str}`, `code 0` = success.
- `code = HTTP*100+seq` (e.g. `40401` → HTTP 404). Each response carries `X-Request-ID`;
  server logs `rid=<id> <METHOD> <PATH> -> <status> (<dur>)`.
- Optional auth 可选鉴权：set `GSM_API_TOKEN` → all routes require
  `Authorization: Bearer <token>`, else 401. Unset = open LAN mode (WARNING at boot).
- `404/405` are JSON envelopes under `/api/*`; legacy root 404 stays plain-text.
- v1 JSON bodies must contain exactly one JSON object. `POST /api/v1/cell` is
  limited to 1 MiB; other v1 JSON bodies are limited to 64 KiB. The limit
  includes trailing data; overflow returns `413/41301`.
  v1 JSON 请求体必须且只能包含一个 JSON 对象；小区启动上限 1 MiB，其余请求上限
  64 KiB，尾随数据也计入限制，超限返回 `413/41301`。

## 1. Cell 小区

### POST /api/v1/cell — start 启动

Explicit GSM params (no silent fallback 无静默回退). `{}` reuses `last_start.json`.

```bash
curl -X POST http://127.0.0.1:8082/api/v1/cell -H 'Content-Type: application/json' \
  -d '{"arfcns":"1","c0":"540","band":"1800","mcc":"001","mnc":"01","lac":"4420","ci":"41240","short_name":"test","network":"eth0"}'
# 200 {"code":0,"message":"cell started",...}
curl -X POST http://127.0.0.1:8082/api/v1/cell -d '{}'  # reuse profile 复用存档
```

| Case | HTTP | code | message |
|---|---|---|---|
| success | 200 | 0 | `cell started` |
| malformed JSON | 400 | 40001 | `malformed JSON body` |
| body over 1 MiB | 413 | 41301 | `request body too large` |
| validation | 422 | 42201 | `validation failed` (`data.errors:[{field,reason}]`) |
| already running | 409 | 40901 | `cell already running` |
| no SDR | 503 | 50301 | `no SDR device attached` |
| unknown iface | 422 | 42201 | `unknown network interface "x"` |
| other | 500 | 50001 | `start failed: <reason>` |

Strictness vs legacy 严格化差异：`band` must be `900/1800` (else 422);
`mcc` 3 digits, `mnc` 2-3 digits; `network` metachar-rejected; unknown JSON
fields rejected (→ same 400 as malformed body).

### GET /api/v1/cell — status 状态

`200 {"code":0,"data":{"running":bool,"openbts":bool,"transceiver":bool,...}}`.

### DELETE /api/v1/cell — stop 停止 (idempotent 幂等)

Running → `200 stopped:true`; idle → `200 stopped:false` (legacy returned failure; v1 succeeds).
If any OpenBTS/transceiver/sipauthserve/smqueue/Asterisk process remains after
the stop attempt, the API returns `500` instead of reporting a false success.
停止后若任一小区相关进程仍存活，接口返回 `500`，不再误报停止成功。

## 2. UE 终端

### GET /api/v1/ue

```json
{"code":0,"message":"ok","data":{"ues":[{"imsi":"...","imei":"...","number":"...","ip":"..."}],"count":1}}
```

Missing fields are `null`. `412 cell not running`, `404 no UE attached`.

## 3. SMS 短信

### GET /api/v1/sms — list from smqueue log

`200 {sms[],count}`; `404 no SMS yet / log not found`.
Note: Chinese content not retrievable from log (OpenBTS limitation, documented).

### POST /api/v1/sms — send 发送

Body: `{"imsi":"15 digits","sender":"...","text":"..."}` (`smsmessage` alias accepted).
Non-GSM7 (e.g. Chinese) → `422` with explicit reason (legacy passed through and failed opaquely).

## 4. Subscribers 签约用户 (explicit 显式语义)

GSM has no SIM-writer API; SIMs are provisioned externally. The `user_db`
counterpart is the OpenBTS subscriber registry
(`TMSITable.db` volatile + `sqlite3.db` persistent). v1 makes add/query explicit —
no implicit side effects.

- `GET /api/v1/subscribers` → `200 {subscribers:[{imsi,number}]}`;
  missing registry DB `404`, query failure `500`（签约库缺失返回 `404`，查询失败返回 `500`）。
- `POST /api/v1/subscribers {"imsi":"...","number":"..."}` → `200 subscriber updated`;
  `404 imsi not found (attach UE first)`; `412 not in asterisk registry`;
  `500 subscriber update failed`. The three TMSI/Asterisk writes use one
  transaction and roll back on SQL or affected-row-count failure.
  三处 TMSI/Asterisk 写入使用同一事务，SQL 错误或受影响行数异常时回滚。

Legacy `POST /setphonenumber` frozen (message_id 3/4 kept).

## 5. Config 配置

- `GET /api/v1/config` → `200 {config:[{key,value}]}`; `404 db not found`;
  other query failures `500`（其他查询错误返回 `500`）。
- `PATCH /api/v1/config {"name":"GSM.Identity.ShortName","value":"test"}` →
  `200 config updated`; cell must be stopped else `409`; missing DB `404`;
  bad name/value `422`.
- `POST /api/v1/network {"iface":"eth0"}` → MASQUERADE for `192.168.99.0/24`.

## 6. Profile & health 存档与健康

- `GET /api/v1/profile` → `200 {has_profile,profile?,ues[]}` (no secrets).
- `GET /api/v1/health` → `200 {ok,running,cell,sdr,time}`.

## 7. Error codes 错误码

| code | HTTP | meaning 含义 |
|---|---|---|
| 0 | 200 | success (incl. idempotent stop) |
| 40001 | 400 | malformed body |
| 40101 | 401 | bad/missing bearer token (only when set) |
| 40401 | 404 | not found (path/id/no UE/no SMS/no DB) |
| 40501 | 405 | method not allowed |
| 40901 | 409 | conflict (cell running / config while running) |
| 41201 | 412 | precondition (cell not running / not in registry) |
| 41301 | 413 | JSON request body too large / JSON 请求体超限 |
| 42201 | 422 | validation failed (`data.errors`) |
| 50001 | 500 | internal (message carries reason) |
| 50301 | 503 | no hardware (no SDR) |

## 8. Legacy↔v1 map 对照表

| Legacy 旧 (frozen) | v1 新 | Behavior change 差异 |
|---|---|---|
| `POST /start` | `POST /api/v1/cell` | strict validation; unknown band 422 (was silent/preset) |
| `POST /stop` | `DELETE /api/v1/cell` | idle stop now idempotent success |
| `POST /ueinfo` | `GET /api/v1/ue` | array rows → objects; no-UE → 404 |
| `POST /smsinfo` | `GET /api/v1/sms` | same source; structured objects |
| `POST /sendsms` | `POST /api/v1/sms` | non-GSM7 → explicit 422 |
| `POST /setphonenumber` | `POST /api/v1/subscribers` | explicit upsert + 404/412 |
| `POST /config {id}` | `POST /api/v1/cell` (explicit) | presets kept server-side; new callers send explicit fields |
| `POST /getconfig` | `GET /api/v1/config` | objects instead of pairs |
| `POST /allconfig` | `PATCH /api/v1/config` | running-cell guard → 409 |
| `POST /iptables` | `POST /api/v1/network` | same MASQUERADE, validated iface |
