# gsm-system API 2.1 / API 2.1 规范

Machine-readable contract / 机器可读契约：[`api/openapi.yaml`](api/openapi.yaml)
Postman: [`../postman/gsm-system.postman_collection.json`](../postman/gsm-system.postman_collection.json)
Usage and result interpretation / 使用与结果解读：
[`../postman/README.md`](../postman/README.md) ·
[Operations / 操作与排障](OPERATIONS.md)

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
trailing JSON are rejected. Cell start permits 1 MiB, preset create/update
permits 16 KiB, and every other JSON mutation permits 64 KiB. JSON 请求体必须且只能
是一个对象；拒绝未知字段、`null`、数组与尾随 JSON；小区启动上限 1 MiB，
预设创建/更新上限 16 KiB，其余写接口上限 64 KiB。

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
`/healthz`, `/status`, `/profile`, `/preset`, `/presets`) are not registered and
return `404` envelopes.
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
  "network": "eth0"
}
```

Exactly one of the following forms is accepted / 仅接受以下三种形式之一：

1. the complete explicit object above / 上述完整显式参数；
2. `{"preset_id":"0"}` to resolve a stored preset / 按 ID 使用存储预设；
3. `{}` to reuse the existing valid last profile / 复用已有有效的上次存档。

`preset_id` is mutually exclusive with every explicit cell field. A missing
preset returns `404`; mixed or invalid input returns `422`. Starting from a
preset copies its `params` into the normal start path and last-profile storage.
It does not modify the preset. `preset_id` 与所有显式小区字段互斥；预设不存在
返回 `404`，混合或无效输入返回 `422`。通过预设启动只解析其 `params`，
不改写预设。

The nine explicit fields form one complete custom profile: partial objects are
not patches. There is no preset-plus-override mode. `{}` is a third, distinct
mode and succeeds only when a complete valid last profile has already been
saved. 九个显式字段必须整体提交，不是局部补丁；不支持“预设 +
字段覆盖”。`{}` 是独立的复用模式，仅在已保存完整有效存档时成功。

Validation / 校验：

- `arfcns` must equal `"1"`;
- band `"900"`: `c0=0..124` or `975..1023`;
- band `"1800"`: `c0=512..885`;
- `mcc`: exactly 3 digits; `mnc`: 2 or 3 digits;
- `lac=1..65279`; `ci=0..65535`;
- `network` is required and names an interface **inside the container**, not a
  host NIC. The default Compose bridge interface is `eth0`; existence is not a
  schema-level check. `network` 必填，它指定容器内网卡（默认 `eth0`），
  而非宿主机网卡。

Success: HTTP `200`, message `cell started`,
`data:{"band":"...","mcc":"...","mnc":"...","short_name":"..."}`.
Important failures: `409` running/transitioning, `422` invalid profile,
`404` missing preset, `503` SDR unavailable, `500` process startup failure.

#### Postman manual start / Postman 手动启动

1. Re-import the current collection, then use your configured environment or
   the **gsm-system 2.1 example / 环境示例** template so `baseUrl` points at
   the chosen host. Configure `token` only when Bearer auth is enabled. Existing `enable_mutations` and
   `enable_rf_start` environment values may be deleted or left in place; they
   no longer have any effect. / 必须重新导入新集合；环境可沿用
   已配置环境或参照示例，`token` 仅在启用鉴权时配置，旧开关值可删除
   或保留为无效值。
2. The groups are **Read-only / 查询接口**, **Cell start /
   小区启动（二选一）**, and **Mutations / 写操作（手动发送）**. Running only
   the GET group provides a read-only check; mutation requests are sent
   directly when **Send** is pressed. / 仅运行 GET 分组可做只读检查；
   写请求点击 **Send** 后直接发送。
3. Open **Cell start / 小区启动（二选一）** and press
   **Send** on exactly one clearly named request: **POST /cell — Preset /
   按预设启动** uses `start_preset_id` (default `0`, any stored ID is
   allowed); **POST /cell — Custom / 自定义参数启动** uses all of
   `arfcns`, `band`, `c0`, `mcc`,
   `mnc`, `lac`, `ci`, `short_name`, and `iface`. `default_preset_id` remains a
   read-only default-query variable, while `preset_id` remains a preset-CRUD
   fixture. / 在该分组中仅发送 preset 或 custom 其中
   一个；三个 preset 相关变量用途不同，不要混用。
4. The start pre-request check remains active: unresolved variables, an
   empty/mixed JSON body, or an incomplete custom body are explained in the
   Postman Console and skipped before any RF request is sent. This uses Postman's documented
   [`pm.execution.skipRequest()` behavior](https://learning.postman.com/docs/tests-and-scripts/write-scripts/postman-sandbox-reference/pm-execution/).
   / 启动请求仍会检查 JSON 参数，并在 Console 说明跳过原因。
5. Never run the full collection: cell start, deletion, and SMS submission
   have real effects, with no additional enable switch. `verify_factory_defaults`
   is an independent response-assertion option and does not control whether a
   request is sent. / 不要 Run 整个集合：启动、删除和发短信会真实执行，
   且没有额外开关；`verify_factory_defaults` 仅控制响应断言。
6. A sent start may take up to 90 seconds. When it returns, send `GET /cell` and
   inspect `state`, `ready`, `sms_ready`, and `voice_ready`; HTTP success alone
   is not RF or handset acceptance. / 已发出的启动最长可等待 90 秒；返回后
   发送 `GET /cell` 查询状态，HTTP 成功不等于 RF/真机验收。

### `DELETE /cell`

Idempotent / 幂等：HTTP `200`,
`data:{"stopped":true}` when a running stack was stopped, or `false` when it was
already stopped. TERM precedes KILL to allow CDR flush. 若已停止仍返回成功。

## 3. Presets / 预设

Presets are non-secret named cell profiles. A new data volume is initialized
with the five editable defaults below. They persist at `/data/presets.json`
using the same private, atomic-file discipline as the last profile. 预设是不含
密钥的命名小区参数；新数据卷初始化五套可编辑默认预设，并持久化于
`/data/presets.json`。

| ID | `name` / `short_name` | `band` | `c0` | `mcc` | `mnc` |
|---|---|---:|---:|---:|---:|
| `"0"` | `addx` | `1800` | `540` | `001` | `01` |
| `"1"` | `ChinaMobile` | `900` | `55` | `460` | `00` |
| `"2"` | `ChinaMobile` | `1800` | `540` | `460` | `00` |
| `"3"` | `ChinaUnicom` | `900` | `70` | `460` | `01` |
| `"4"` | `ChinaUnicom` | `1800` | `668` | `460` | `01` |

Every default also has `arfcns="1"`, `lac="1"`, `ci="1"`, and
`network="eth0"`. Operators must still verify that the selected band/channel is
legal and suitable before explicitly starting RF. 每套默认配置均使用单载频、
`lac/ci="1"` 及容器网卡 `eth0`；显式启动射频前仍须确认合法频段和信道。

Preset IDs are stable slugs matching `^[a-z0-9][a-z0-9_-]{0,63}$`.
预设 ID 是稳定 slug，仅允许小写字母、数字、下划线和连字符，最长 64 字符。

### `GET /presets`

HTTP `200`; no pagination or query parameters / 不分页，不接受查询参数：

```json
{"items":[{"id":"0","name":"addx","description":"","params":{"arfcns":"1","c0":"540","band":"1800","mcc":"001","mnc":"01","lac":"1","ci":"1","short_name":"addx","network":"eth0"}}]}
```

The example abbreviates the array to one item; a fresh store returns all five
defaults. 示例仅展示一项结构，新预设库实际返回五项。

Each item is a full preset object. A fresh installation returns the five
defaults above, sorted by ID. 每项都是完整预设对象；新安装按 ID 排序返回
上述五套默认配置。

### `POST /presets`

```json
{
  "id": "lab-900",
  "name": "Lab 900",
  "description": "Indoor fixture",
  "params": {
    "arfcns": "1", "c0": "55", "band": "900",
    "mcc": "001", "mnc": "01", "lac": "1", "ci": "1",
    "short_name": "LAB", "network": "eth0"
  }
}
```

`id`, `name`, and `params` are required; omitted `description` is stored as an
empty string. `name` is 1..128 UTF-8 bytes, `description` is at most 2048 UTF-8
bytes, and `params` must contain every valid explicit `CellStart` field.
Success is HTTP `201`, returns the created preset object in
`data`, and sets `Location: /api/v1/presets/lab-900`. Duplicate ID: `409`;
invalid ID, metadata, or params: `422`. `id`、`name` 和完整 `params` 必填；创建成功返回
`201` 及 `Location`；`description` 可省略并存为空字符串；名称最长 128 UTF-8
字节、说明最长 2048 UTF-8 字节；ID 重复返回 `409`，校验失败返回 `422`。

### `GET /presets/{id}`

HTTP `200` returns `data` as the preset object itself (not a nested `preset`
property). Missing ID: `404`; invalid slug: `422`. HTTP `200` 的 `data` 直接是
预设对象；不存在返回 `404`，ID 格式错误返回 `422`。

### `PUT /presets/{id}`

```json
{
  "name": "Lab 900 updated",
  "description": "Updated fixture",
  "params": {
    "arfcns": "1", "c0": "60", "band": "900",
    "mcc": "001", "mnc": "01", "lac": "1", "ci": "1",
    "short_name": "LAB", "network": "eth0"
  }
}
```

The path ID is immutable: the body accepts only `name`, optional `description`,
and a complete `params` object; a body `id` is rejected. Omitted `description`
becomes empty because PUT replaces the resource. HTTP `200` returns the
updated preset object. Missing ID: `404`; invalid input: `422`. 路径 ID 不可变，
请求体不接受 `id`；更新成功返回完整预设。

### `DELETE /presets/{id}`

HTTP `200`, `data:{"deleted":true}`. A missing preset returns `404`.
删除成功返回 `{"deleted":true}`；不存在返回 `404`。

Creating, editing, or deleting a preset never starts, stops, or reconfigures a
running cell. Deleting the preset used for a previous start does not change the
running cell or `/data/last_start.json`. There is no autostart. 预设 CRUD 不会
启动、停止或重配当前小区；删除曾用预设也不影响正在运行的小区或
上次启动存档，且不存在自动启动。

The store accepts at most 256 presets. Its versioned on-disk document is capped
at 1 MiB and kept sorted. The one-time v1→v2 upgrade adds only missing default
IDs and never overwrites a user value already stored under `"0"`..`"4"`.
After the file is v2, user edits and deletions are authoritative and defaults
are not regenerated. Corruption, unsupported versions, or I/O failure return
`50001` rather than silently resetting data. 预设最多 256 条，磁盘文件上限
1 MiB 并按 ID 排序；v1 仅在一次性升级为 v2 时补齐缺失默认 ID，不覆盖
同 ID 用户内容；v2 后用户修改或删除均不会重生。损坏、不支持版本或 I/O
错误返回 `50001`，不会静默重置。

Management-plane smoke test (CRUD plus duplicate/missing/invalid cases, no
valid cell start and no RF) / 管理面预设冒烟测试（不启动 RF）：

```powershell
$env:GSM_API_TOKEN = "TOKEN" # omit when authentication is disabled / 未开鉴权时省略
.\scripts\tests\presets_smoke.ps1 -BaseUrl http://HOST:8082/api/v1
```

The script always removes its unique fixture in `finally`. 脚本会在 `finally` 中清理唯一测试预设。

An isolated image-level defaults/persistence/start-routing check is available on the
Ubuntu SDR server only: `./scripts/tests/test_presets_image.sh IMAGE`. It uses a
temporary volume, no published port/USB/privilege, and a failing mock UHD probe,
so both start modes stop at `503` before RF. / 镜像级测试仅在 Ubuntu SDR
服务器执行，不发射。

## 4. Configuration and profile / 配置与存档

### `GET /config`

HTTP `200`: `data.config` is an array of `{"key":"KEY","value":"VALUE"}`.
Missing database: `404`; query failure: `500`.

### `PATCH /config`

Updates one or more allowlisted radio/identity values while the cell is stopped.
The exact writable keys are `GSM.Radio.ARFCNs`, `GSM.Radio.C0`,
`GSM.Radio.Band`, `GSM.Identity.MCC`, `GSM.Identity.MNC`, `GSM.Identity.LAC`,
`GSM.Identity.CI`, and `GSM.Identity.ShortName`:

```json
{"values":{"GSM.Radio.Band":"900","GSM.Radio.C0":"55"}}
```

HTTP `200`: `data.updated` is an array of updated key names. Empty/invalid values
return `422`; a running/transitioning cell returns `409`; missing DB returns
`404`. `GET /config` exposes the native database, but other keys—including
`Control.LUR.*` welcome/admission settings—are read-only through this API.
仅允许原子更新上述八个无线与标识键；小区运行或切换中返回 `409`。`GET /config`
虽可读取原生数据库，但 `Control.LUR.*` 欢迎/接入配置等其他键不能通过本 API 修改。

### `GET /profile`

HTTP `200`: `data:{"has_profile":false}` or
`data:{"has_profile":true,"profile":{...CellStart fields...}}`. Subscriber and
connection data are not embedded. 不内嵌连接或签约列表。

## 5. Connections and subscribers / 连接与签约

### `GET /connections`

HTTP `200` even when empty:

```json
{
  "connections": [
    {
      "imsi":"IMSI",
      "imei":null,
      "number":null,
      "number_source":"subscriber_registry",
      "ip":null,
      "auth":0,
      "reject_code":4
    }
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

`imei`, `number`, and `ip` are nullable and use JSON `null`, not an empty
string. `number_source` is an optional nullable field. Its values are
`subscriber_registry`, `openbts_tmsi`, or `inconsistent_registry`. A matching
subscriber-registry row is authoritative: a current binding overrides a stale
or empty native TMSI value, and an authoritative unbind keeps `number:null`
even if the TMSI cache still contains an old number. An inconsistent registry
row also yields `number:null` with `number_source:"inconsistent_registry"`
rather than choosing one conflicting value. If the registry is unavailable or
has no matching IMSI, the observed TMSI value is used and identified as
`openbts_tmsi`; no evidenced value leaves the source null. The top-level
`source` remains `openbts_tmsi_sgsn` because rows and attachment diagnostics
still originate from the live observation snapshot.

`auth` and `reject_code` are additive, optional response fields during
a rolling 2.1 upgrade; when present they are nullable integers that preserve
the native values. AUTH maps to `0=unauthorized`, `1=registrar-authorized`,
`2=open-registration`, and `3=fail-open`; none of these recorded values is a
real-time online indicator. In particular, `auth:0` is not authorized.
`reject_code:4` is a native registration rejection result with more than one
possible cause, so it does not by itself diagnose a Ki mismatch. The parser
reads `number` from its fixed native column and the packet-data address from
`SGSN.IPs=`; unrelated tokens such as `WELCOME_SENT` are not phone numbers.

Rows originate from volatile TMSI/SGSN observations and may represent
registration attempts without an allocated TMSI. They are not an admitted or
online-device list. `ip:null` only means that no handset packet-data address was
observed: it does not prove a NAT failure. A handset data IP is expected only
after GPRS is enabled and PDP/SGSN setup succeeds; SMS uses signalling and does
not depend on that handset IP. Native CLI/read failures return `500`; a stopped
OpenBTS process returns `412`; an empty result is normal `200`.

`imei`、`number`、`ip` 均可为 `null`。可选可空的 `number_source` 取值为
`subscriber_registry`、`openbts_tmsi` 或 `inconsistent_registry`。匹配到
签约库行时以签约库为准：新绑定覆盖未刷新或为空的 TMSI 号码，权威解绑也会覆盖
TMSI 遗留号码而保持 `number:null`；签约库不一致时不猜测冲突值，返回
`number:null` 和 `inconsistent_registry`。签约库不可用或无匹配 IMSI 时才使用
TMSI 观察值；无证据时来源为 `null`。顶层 `source` 仍为
`openbts_tmsi_sgsn`，表示连接行及附着诊断的观察来源。

滚动升级期间新增的 `auth`、
`reject_code` 可缺省，存在时为可空整数。AUTH 原生值为
`0=未授权`、`1=Registrar 授权`、`2=开放注册`、`3=失败时开放`，均不证明实时
在线；`reject_code:4` 可能对应多种注册失败原因，不能单凭该值断定
Ki 错误。号码按原生固定列解析，`WELCOME_SENT` 等状态字段不会被当作号码。
TMSI/SGSN 行也可能只是未分配 TMSI 的注册尝试，不是已入网或在线名单。
`ip:null` 仅表示未观察到手机分组数据地址，不证明 NAT 失败；手机数据 IP 需在
GPRS 开启且 PDP/SGSN 就绪后才会出现，而短信走信令，不依赖手机数据 IP。

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
`101`/`411` (SMS services), `111` (local voicemail), `112`/`911` (reserved
emergency codes), and `2600`/`2602` (local voice diagnostics) are reserved and
rejected with HTTP 422. 这些服务号码不可绑定到手机。The lab has no PSTN
interconnection or real emergency-calling guarantee.
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

## 6. SMS / 短信

### `GET /sms`

HTTP `200`, including an empty current-start observation window:

```json
{
  "sms": [{
    "time":null,
    "text":null,
    "sender_number":null,
    "sender_imsi":null,
    "receiver_number":null,
    "receiver_imsi":null,
    "identity_resolution":{
      "sender_number":"unknown",
      "sender_imsi":"unknown",
      "receiver_number":"unknown",
      "receiver_imsi":"unknown"
    }
  }],
  "count":1,
  "total":1,
  "limit":100,
  "offset":0,
  "source":"smqueue.log",
  "scope":"current_start",
  "timezone":"Asia/Shanghai",
  "session":{
    "id":"SESSION_ID",
    "started_at":"2026-09-08T16:00:00+08:00",
    "ended_at":null,
    "state":"running"
  },
  "window":{"bytes":1024,"max_bytes":8388608,"truncated":false},
  "truncated":false
}
```

The scope is always `current_start`; there is no all-history query switch.
Only observations within the latest known cell-start boundary are returned.
Stopping or degrading a cell preserves that round until the next accepted start;
failure of an accepted start attempt clears the observation session. Parameter
validation rejection and duplicate-start `409` preserve the existing boundary.
Recreating the management process does not
adopt old log history: with no known start, return `200`, `sms: []`, `count: 0`,
`total: 0`, and `session: null`, even if an old log exists. Original logs are not
deleted. One rename rotation to `.1` is supported. If the boundary is lost or
cannot be verified (for example, after multiple rotations), return `200` with
empty observations and `session.state: boundary_lost`,
never another log's history; genuine I/O failures such as permission errors return
`500`. Session states are `starting`, `running`, `stopped`, or `boundary_lost`.
`running` only means the observation window is not frozen; it does not guarantee
cell readiness (query `GET /api/v1/cell`). This scope selects the current log
observation window, not the message's original creation time: messages in a
persistent queue can appear when reprocessed or replayed during this round.
Pagination and the bounded tail window apply after session scoping, not
to all historical messages. `window` retains only `bytes`, `max_bytes`, and
`truncated`; excluded bytes from prior starts do not themselves mean truncation.

`timezone` is the project's startup-configured IANA time zone, default
`Asia/Shanghai` (UTC+08:00), overridable with `TZ` before launching the project.
Native SMS timestamps without an offset are interpreted in that zone. Non-null
message `time` is RFC3339Nano with a zone offset (UTC may use `Z`); session
`started_at` and nullable `ended_at` are RFC3339 timestamps with a zone offset.
These settings control time-zone interpretation/display, not the host clock.

短信范围固定为 `current_start`，没有全历史查询开关。只返回最近一次已知小区启动
边界内的观察；停止或降级后保留该轮，直到下一次已接受的启动尝试。已接受的
启动尝试失败才清空会话；参数校验拒绝或重复启动 `409` 保留现有边界。
管理进程重建后不接管旧历史：无已知启动时，即使存在旧日志，也返回 `200`、
空短信数组、零计数和 `session: null`，不是 `404`，且不删除原日志。
支持一次 rename 轮转至 `.1`；边界丢失或多次轮转后不可验证时，返回 `200`
空观察和 `boundary_lost`，不回退
读取其他历史；真实读取错误（例如权限错误）返回 `500`。会话状态为 `starting`、
`running`、`stopped` 或 `boundary_lost`。
`running` 仅表示观察窗口尚未冻结，不保证小区 ready；就绪状态查询 `GET /api/v1/cell`。
范围依据本次日志观察窗口，而非短信最初创建时刻；持久队列中的短信在本次重新
处理或重放时仍可能出现。
分页和最大读取字节数限制均在本轮范围内生效；上一轮被排除的字节不算截断。
时区在整个项目启动前通过 `TZ` 自定义，默认东八区 `Asia/Shanghai`。
不带时区的原生短信时间按项目时区解析；非空短信时间输出带偏移的 RFC3339Nano，
会话起止时间输出带偏移的 RFC3339，UTC 可使用 `Z`。时区配置不修改宿主机时钟。

New images preferentially emit and
parse the keyed structured NOTICE observation `GSM_SMS_V1` with
`qtag_hex`/`from_hex`/`to_hex`/`text_hex`. Hex encoding preserves the exact
parties and body without allowing message content to inject log lines; the
qtag merges the observation with a matching legacy NOTICE in the same smqueue
process generation. Older `Got SMS rqst qtag ...` events are conservatively
supported for their timestamp and evidenced parties, but an unkeyed
`Decoded text:` line is never associated with them: without message identity,
`text` remains null rather than risking fabricated cross-message content. An
explicitly keyed decoded line may still merge by qtag. Legacy complete local
`Request Message Delivery` / `Decoded text` / `Deliver message` blocks also
remain supported. Records are deduplicated by message identity (qtag scoped to
one smqueue process generation), never by text or party content; two different
messages with identical content remain two records. The six observation fields
keep their existing nullable shape. A missing number or IMSI is completed only
when the other identifier has exactly one globally unique, internally consistent
mapping in the current subscriber registry. Duplicate IMSIs/numbers,
inconsistent/unbound entries, and service shortcodes such as `101` and `411`
remain unresolved. An observed log value is never overwritten. The four fixed
`identity_resolution` keys report `log_observation`,
`current_subscriber_binding`, or `unknown` for each identity field. A
`current_subscriber_binding` value is query-time context and must not be treated
as the binding that existed when the message was logged. These are observations,
not delivery receipts. If the registry is unavailable, current-start observations remain
available and missing identity fields stay `null` with `unknown` provenance.

Only complete entries inside the bounded tail window are exposed;
`window.truncated` and top-level `truncated` report excluded older bytes.
新镜像优先记录并解析结构化 NOTICE `GSM_SMS_V1`，其中 qtag、收发方和正文采用
hex 字段，既能准确恢复值，也避免正文注入伪日志行；同一进程代际内按 qtag 与旧
NOTICE 合并。旧 `Got SMS rqst qtag ...` 事件仅保留有证据的时间与收发方；不带
消息身份的 `Decoded text:` 一律不与其关联，`text` 保持 `null`，不会伪造跨消息
正文。显式携带 qtag 的正文仍可合并，旧版完整局部详细日志块继续支持。去重依据
消息身份/qtag，而非正文或收发方内容，
因此同文不同消息仍分别保留。六个观察字段保持原有可空形状；仅当另一身份在当前
签约库中存在全局唯一且内部一致的映射时，才补全缺失号码或 IMSI。重复 IMSI/号码、
不一致或未绑定记录，以及 `101`、`411` 等服务短码都保持未知；日志观察值绝不覆盖。
固定四键 `identity_resolution` 为每个身份字段标注 `log_observation`、
`current_subscriber_binding` 或 `unknown`。当前绑定只是查询时上下文，不表示短信发生时
的历史绑定。签约库不可读时仍返回本轮日志观察，缺失身份保持 `null`/`unknown`。这些是
日志观察，不是送达回执。日志读取受 `max_history_bytes` 限制。

`GET /sms` observations from patched native smqueue can contain UTF-8 Chinese
decoded from uncompressed UCS-2 `DCS 0x08` / `0x18..0x1b`. The decoder checks
octet lengths and skips bounded UDH bytes; concatenated SMS remain separate
segment observations, not reassembled messages. Malformed payloads, unsupported
DCS and surrogate code units (including UTF-16 emoji pairs) leave text unknown
(`null`), never guessed from raw hex. Native TPDU forwarding preserves original
DCS/UDHI/user-data bytes. This receive-side support does not expand `POST /sms`.

带补丁的原生 smqueue 可将未压缩 UCS-2 `DCS 0x08` / `0x18..0x1b` 中文解码为
UTF-8 日志正文；检查字节长度并跳过有界 UDH，长短信仍按片段记录，不做重组。
畸形数据、不支持的 DCS 和代理码元（含 UTF-16 emoji 对）保持正文未知 `null`，
不从原始 hex 猜测文字；原生 TPDU 转发保留 DCS、UDHI 与原始数据字节。
该接收端能力不扩大下述 `POST /sms` 的字符集。

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

The normal welcome path does not submit an SMS when the configured message is
empty. A native `WELCOME_SENT` scheduling marker records neither API submission
nor handset delivery; likewise, HTTP `202` is not delivery evidence. / 正常欢迎
消息为空时不会提交短信；原生 `WELCOME_SENT` 调度标记不代表已提交或手机已收到，
HTTP `202` 同样不代表送达。

## 7. Calls / 通话

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
are interpreted as UTC and returned as RFC3339 in the startup-configured project
time zone (`TZ`, default `Asia/Shanghai`); native CDR storage remains UTC.
`answered_at`, `unique_id`, and `user_field` are
nullable. Missing CDR: `404`; non-18-column/malformed/time/read failure: `500`.
No records are synthesized. 每项映射真实 CDR 的 18 列，原生时间按 UTC 解析，
输出转换为项目启动时区的 RFC3339；原始 CDR 仍按 UTC 存储，不生成虚假记录。

## 8. Network / 网络

### `GET /network[?iface=eth0]`

With no query, uses the saved profile interface; without either, returns a
precondition/validation error. HTTP `200`:

```json
{"iface":"eth0","rule_present":true,"ipv4_forwarding":true,"persisted":false}
```

Invalid interface: `422`; iptables inspection unavailable: `503`.
`ipv4_forwarding` is `true`, `false`, or `null` when the read-only
`/proc/sys/net/ipv4/ip_forward` value is unavailable. The API never changes this
sysctl. `rule_present` describes only the managed MASQUERADE rule and does not by
itself prove forwarding works. 接口只读转发开关，不修改 sysctl；规则存在不等于转发可用。

### `PUT /network`

```json
{"iface":"eth0"}
```

`iface` is required on `PUT`; only `GET` may fall back to the saved profile.

Checks before adding the managed `192.168.99.0/24` MASQUERADE rule, so repeated
requests do not append duplicates. The cell must be stopped. HTTP `200`:

```json
{"iface":"eth0","rule_present":true,"changed":false,"ipv4_forwarding":true,"persisted":false}
```

`persisted:false` means host firewall resets/reboots may require reapplication.
Running/transitioning cell: `409`; invalid interface: `422`; iptables unavailable
or command failure: `503`. `POST /network` is removed and returns `405` with
`Allow: GET, PUT`.

## 9. Health / 健康

### `GET /health`

```json
{
  "ok":true,
  "version":"2.1.0",
  "revision":"REVISION",
  "cell":{"state":"stopped","ready":false,"sms_ready":false,"voice_ready":false},
  "time":"2026-09-08T08:00:00+08:00"
}
```

The handler does not take the start/stop transition lock and does not run an
exclusive SDR scan. Health remaining responsive during a long transition is by
design. 健康检查不获取启停锁、不独占探测 SDR，因此切换期间仍可响应。
`time` uses RFC3339 in the startup-configured project time zone (`TZ`, default
`Asia/Shanghai`). / `time` 为项目启动时区的带偏移 RFC3339 时间，默认东八区。

HTTP `200` and `ok:true` describe a responsive management handler, even if
`cell.state=degraded`. Inspect the nested cell state separately. The container's
read-only `gsm-system --healthcheck` accepts intentionally stopped or fully ready
running cells; it rejects transitioning/degraded states. Neither check proves
handset delivery, audio quality or Internet connectivity. `/health` uses the same
optional Bearer authentication as other endpoints.
HTTP 200 与 ok:true 仅说明管理面可响应，即使内部小区 degraded 也可能返回成功；
须另外检查 cell 状态。容器探针仅接受停止态或完全就绪的运行态，拒绝切换/降级状态。
两者均不证明真机短信送达、音质或上网；健康接口同样受可选 Bearer 鉴权保护。
