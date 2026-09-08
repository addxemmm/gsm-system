# gsm-system API 2.1 / API 2.1 规范

Machine-readable contract / 机器可读契约：[`api/openapi.yaml`](api/openapi.yaml)
Postman: [`../postman/gsm-system.postman_collection.json`](../postman/gsm-system.postman_collection.json)

## 1. Conventions / 通用约定

- Base URL / 基地址：`http://HOST:8082/api/v1`
- JSON media type: mutations require `Content-Type: application/json`.
- Optional auth / 可选鉴权：when `GSM_API_TOKEN` is configured, send
  `Authorization: Bearer TOKEN`; otherwise omit it.
- Every response is JSON and includes `X-Request-ID` plus:

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "request_id": "REQUEST_ID"
}
```

`code=0` means the HTTP operation succeeded, including HTTP `202`. Error codes
use `HTTP*100+1` (for example `42201`). Error details use
`data.errors:[{"field":"FIELD","reason":"REASON"}]` when applicable.
`data` may be absent when an error has no structured detail.

`code=0` 包括 HTTP `202` 成功；错误码采用 `HTTP*100+1`，字段错误位于
`data.errors`，无结构化详情时可省略 `data`。

JSON bodies must contain exactly one object. Unknown fields, `null`, arrays, and
trailing JSON are rejected. Cell start permits 1 MiB; every other JSON mutation
permits 64 KiB. JSON 请求体必须且只能是一个对象；拒绝未知字段、`null`、数组与尾随 JSON；
小区启动上限 1 MiB，其余写接口上限 64 KiB。

List endpoints (`/connections`, `/subscribers`, `/sms`, `/calls`, and
`/calls/history`) accept `limit` (default `100`, range `1..500`) and `offset`
(default `0`, range `0..1000000`). Unknown or repeated query parameters return
`422`. Each payload contains its named array plus `count` (this page), `total`
(within the observable/bounded source), `limit`, `offset`, `source`, `window`,
and `truncated`. 列表接口统一分页；未知、重复或越界查询参数返回 `422`。

Common errors / 通用错误：

| HTTP | code | Meaning / 含义 |
|---:|---:|---|
| 400 | 40001 | malformed/wrong-shape JSON / JSON 格式或形状错误 |
| 401 | 40101 | missing or bad Bearer token / 令牌缺失或错误 |
| 404 | 40401 | path or resource not found / 路径或资源不存在 |
| 405 | 40501 | method not allowed; inspect `Allow` / 方法不允许 |
| 409 | 40901 | state/resource conflict / 状态或资源冲突 |
| 412 | 41201 | unmet precondition / 前置条件不满足 |
| 413 | 41301 | request body too large / 请求体过大 |
| 415 | 41501 | unsupported media type / 媒体类型错误 |
| 422 | 42201 | validation failed / 字段校验失败 |
| 500 | 50001 | internal/native command or data error / 内部错误 |
| 503 | 50301 | hardware/native service unavailable / 硬件或原生服务不可用 |

Retired root routes (`/start`, `/stop`, `/ueinfo`, `/smsinfo`, `/sendsms`,
`/setphonenumber`, `/config`, `/getconfig`, `/allconfig`, `/iptables`,
`/healthz`, `/status`, `/profile`) are not registered and return `404` envelopes.
旧根路径全部移除并返回标准 `404` 包络。

## 2. Cell / 小区

### `GET /cell`

Returns process lifecycle state without an exclusive SDR probe. 不独占探测 SDR。

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "state": "running",
    "ready": true,
    "sms_ready": true,
    "voice_ready": true,
    "transitioning": false,
    "running": true,
    "openbts": true,
    "transceiver": true,
    "sipauthserve": true,
    "smqueue": true,
    "asterisk": true,
    "started_at": "2026-09-08T00:00:00Z"
  },
  "request_id": "REQUEST_ID"
}
```

`state` is `stopped|transitioning|degraded|running`. `ready` means all five
managed processes are alive, not RF/handset acceptance. `started_at` is optional.
`ready` 只代表五个托管进程存活，不代表射频或真机验收。

### `POST /cell`

```json
{
  "arfcns": "1",
  "c0": "C0",
  "band": "1800",
  "mcc": "MCC",
  "mnc": "MNC",
  "lac": "LAC",
  "ci": "CI",
  "short_name": "LAB",
  "network": "IFACE"
}
```

An empty `{}` reuses an existing valid profile. It does not select a hidden
preset. `{}` 仅复用已有有效存档。

Validation / 校验：

- `arfcns` must equal `"1"`;
- band `"900"`: `c0=0..124` or `975..1023`;
- band `"1800"`: `c0=512..885`;
- `mcc`: exactly 3 digits; `mnc`: 2 or 3 digits;
- `lac=1..65279`; `ci=0..65535`;
- `network` is a safe Linux interface name; existence is not a schema-level check.

Success: HTTP `200`, message `cell started`,
`data:{"band":"...","mcc":"...","mnc":"...","short_name":"..."}`.
Important failures: `409` running/transitioning, `422` invalid profile,
`503` SDR unavailable, `500` process startup failure.

### `DELETE /cell`

Idempotent / 幂等：HTTP `200`,
`data:{"stopped":true}` when a running stack was stopped, or `false` when it was
already stopped. TERM precedes KILL to allow CDR flush. 若已停止仍返回成功。

## 3. Configuration and profile / 配置与存档

### `GET /config`

HTTP `200`: `data.config` is an array of `{"key":"KEY","value":"VALUE"}`.
Missing database: `404`; query failure: `500`.

### `PATCH /config`

Updates one or more native OpenBTS values while the cell is stopped:

```json
{"values":{"GSM.Radio.Band":"900","GSM.Radio.C0":"55"}}
```

HTTP `200`: `data.updated` is an array of updated key names. Empty/invalid values
return `422`; a running/transitioning cell returns `409`; missing DB returns
`404`. 更新多个键；小区运行或切换中返回 `409`。

### `GET /profile`

HTTP `200`: `data:{"has_profile":false}` or
`data:{"has_profile":true,"profile":{...CellStart fields...}}`. Subscriber and
connection data are not embedded. 不内嵌连接或签约列表。

## 4. Connections and subscribers / 连接与签约

### `GET /connections`

HTTP `200` even when empty:

```json
{
  "connections": [
    {"imsi":"IMSI","imei":null,"number":null,"ip":null}
  ],
  "count": 1,
  "total": 1,
  "limit": 100,
  "offset": 0,
  "source": "openbts_tmsi_sgsn",
  "window": "current_snapshot",
  "truncated": false
}
```

Nullable fields are JSON `null`, not an empty string. Native CLI/read failures
return `500`; a stopped OpenBTS process returns `412`. Empty results are normal
`200`. This is a current observation and does not label subscribers online.
空列表是正常 `200`；可空字段使用 JSON `null`；连接观察不等于在线判定。

### `GET /subscribers`

```json
{
  "subscribers": [
    {"imsi":"IMSI","number":null,"binding_consistent":true}
  ],
  "count": 1,
  "total": 1,
  "limit": 100,
  "offset": 0,
  "source": "asterisk_registry",
  "window": "full",
  "truncated": false
}
```

Missing registry DB: `404`; duplicate/inconsistent native rows: `409`; other
query failures: `500`.

### `GET /subscribers/{imsi}`

HTTP `200`: `data.subscriber` has `imsi`, nullable `number`, and
`binding_consistent`. Invalid IMSI: `422`; absent subscriber: `404`.

### `PUT /subscribers/{imsi}/number`

```json
{"number":"NUMBER"}
```

Binds/replaces one number transactionally in persistent native registries.
`111` (local voicemail) and `112`/`911` (reserved emergency codes) cannot be
bound. The lab has no PSTN interconnection or real emergency-calling guarantee.
HTTP `200`:

```json
{
  "subscriber":{"imsi":"IMSI","number":"NUMBER","binding_consistent":true},
  "tmsi_projection":"updated"
}
```

`tmsi_projection` is `updated|not_present|unavailable|failed`; it reports the
best-effort volatile TMSI projection separately from persistent commit.
Important failures: invalid IMSI/number `422`, subscriber missing `404`, number
already owned/conflict `409`, transactional failure `500`.

持久库事务提交与易失 TMSI 投影分开报告；号码冲突返回 `409`。

### `DELETE /subscribers/{imsi}/number`

Idempotently removes number routing without deleting the subscriber or writing
the SIM. HTTP `200` returns the same structure with `subscriber.number:null`.
Invalid IMSI `422`; missing subscriber `404`; native transaction failure `500`.
解绑不删除签约、不写 SIM；已无号码时也成功。

`POST /subscribers` is removed and returns `405` with `Allow: GET`.

## 5. SMS / 短信

### `GET /sms`

HTTP `200`, including an empty parsed log:

```json
{
  "sms": [{
    "time":null,
    "text":null,
    "sender_number":null,
    "sender_imsi":null,
    "receiver_number":null,
    "receiver_imsi":null
  }],
  "count":1,
  "total":1,
  "limit":100,
  "offset":0,
  "source":"smqueue.log",
  "window":{"bytes":1024,"max_bytes":8388608,"truncated":false},
  "truncated":false
}
```

Missing log: `404`; read failure: `500`. All six empty message fields are JSON
`null`. Only complete entries inside the bounded tail window are exposed;
`window.truncated` and top-level `truncated` report excluded older bytes.
六个字段的空值均为 `null`，日志读取受 `max_history_bytes` 限制。

### `POST /sms`

```json
{"imsi":"IMSI","sender":"SENDER","text":"hello"}
```

HTTP **`202`**, message `sms submitted`:

```json
{"status":"submitted","imsi":"IMSI"}
```

The acknowledgement means the upstream OpenBTS CLI accepted submission; it does
not prove delivery. Supported text is the safe ASCII intersection of the GSM
default basic alphabet, maximum 159 bytes. UCS-2/Chinese, GSM extension-table
characters, single/double quotes, and backticks are rejected with `422`.
Cell/SMS service not ready: `412`; upstream rejection/CLI failure: `500`.

`202` 仅表示上游接受提交，不代表送达。仅安全 ASCII/GSM 默认基本表交集，最长 159
字节；拒绝中文/UCS-2、扩展表、单双引号与反引号。

## 6. Calls / 通话

### `GET /calls`

Returns active Asterisk channels:

```json
{
  "calls":[{
    "channel":"CHANNEL",
    "context":"CONTEXT",
    "extension":"EXTENSION",
    "state":"STATE",
    "application":"APPLICATION",
    "caller_id":"CALLER_ID",
    "duration_seconds":12,
    "bridge_id":null,
    "unique_id":null
  }],
  "count":1,
  "total":1,
  "limit":100,
  "offset":0,
  "source":"asterisk_active_channels",
  "window":"current_snapshot",
  "truncated":false
}
```

`count` is active returned **channels**, not lifetime calls or necessarily bridged
call pairs. Asterisk CLI unavailable/failing: `503`. `count` 是活动通道条目数。
`bridge_id` is Asterisk's bridge UUID when present, not a bridged-channel name.

### `GET /calls/history`

Reads the real Asterisk CSV CDR (`Master.csv`):

```json
{
  "calls":[{
    "account_code":"ACCOUNT_CODE",
    "source":"SOURCE",
    "destination":"DESTINATION",
    "destination_context":"CONTEXT",
    "caller_id":"CALLER_ID",
    "channel":"CHANNEL",
    "destination_channel":"DESTINATION_CHANNEL",
    "last_application":"APPLICATION",
    "last_data":"DATA",
    "started_at":"RFC3339_TIME",
    "answered_at":null,
    "ended_at":"RFC3339_TIME",
    "duration_seconds":20,
    "billed_seconds":0,
    "disposition":"NO ANSWER",
    "ama_flags":"DOCUMENTATION",
    "unique_id":null,
    "user_field":null
  }],
  "count":1,
  "total":1,
  "limit":100,
  "offset":0,
  "source":"Master.csv",
  "window":{"bytes":2048,"max_bytes":8388608,"truncated":false},
  "truncated":false
}
```

Each item maps all 18 configured `cdr_csv` columns. The three native GMT values
are returned as RFC3339 UTC. `answered_at`, `unique_id`, and `user_field` are
nullable. Missing CDR: `404`; non-18-column/malformed/time/read failure: `500`.
No records are synthesized. 每项映射真实 CDR 的 18 列，时间统一为 RFC3339 UTC；
不生成虚假记录。

## 7. Network / 网络

### `GET /network[?iface=IFACE]`

With no query, uses the saved profile interface; without either, returns a
precondition/validation error. HTTP `200`:

```json
{"iface":"IFACE","rule_present":true,"ipv4_forwarding":true,"persisted":false}
```

Invalid interface: `422`; iptables inspection unavailable: `503`.
`ipv4_forwarding` is `true`, `false`, or `null` when the read-only
`/proc/sys/net/ipv4/ip_forward` value is unavailable. The API never changes this
sysctl. `rule_present` describes only the managed MASQUERADE rule and does not by
itself prove forwarding works. 接口只读转发开关，不修改 sysctl；规则存在不等于转发可用。

### `PUT /network`

```json
{"iface":"IFACE"}
```

`iface` is required on `PUT`; only `GET` may fall back to the saved profile.

Checks before adding the managed `192.168.99.0/24` MASQUERADE rule, so repeated
requests do not append duplicates. The cell must be stopped. HTTP `200`:

```json
{"iface":"IFACE","rule_present":true,"changed":false,"ipv4_forwarding":true,"persisted":false}
```

`persisted:false` means host firewall resets/reboots may require reapplication.
Running/transitioning cell: `409`; invalid interface: `422`; iptables unavailable
or command failure: `503`. `POST /network` is removed and returns `405` with
`Allow: GET, PUT`.

## 8. Health / 健康

### `GET /health`

```json
{
  "ok":true,
  "version":"2.1.0",
  "revision":"REVISION",
  "cell":{"state":"stopped","ready":false,"sms_ready":false,"voice_ready":false},
  "time":"2026-09-08T00:00:00Z"
}
```

The handler does not take the start/stop transition lock and does not run an
exclusive SDR scan. Health remaining responsive during a long transition is by
design. 健康检查不获取启停锁、不独占探测 SDR，因此切换期间仍可响应。
