# SIM and subscribers / SIM 与签约

Release 2.1 has no SIM-writer endpoint. SIMs are provisioned externally; the API
exposes only non-secret identity/number state. Ki and other authentication
material are never returned. 2.1 不提供写卡接口，也不返回 Ki 等鉴权材料。

## State model / 状态模型

| Resource / 资源 | Source / 来源 | Meaning / 含义 |
|---|---|---|
| `GET /api/v1/connections` | volatile `/var/run/TMSITable.db` plus live CLI observations | current observed attach data; not a durable online/offline assertion / 当前观察值，不是持久在线判定 |
| `GET /api/v1/subscribers` | persistent Asterisk `sqlite3.db` | provisioned identities and authoritative number bindings / 签约与权威号码绑定 |
| `GET /api/v1/subscribers/{imsi}` | persistent Asterisk `sqlite3.db` | one subscriber; `number` may be `null` / 单条签约 |

The persistent database lives at `/data/state/asterisk/sqlite3.db` and is exposed
to Asterisk at its native path. The TMSI database is volatile and may not contain
a disconnected subscriber. 持久签约库与易失 TMSI 表用途不同。

## Bind a number / 绑定号码

```http
PUT /api/v1/subscribers/IMSI/number
Content-Type: application/json

{"number":"NUMBER"}
```

- `IMSI` is exactly 15 decimal digits; the subscriber must already exist in both
  required Asterisk registry rows.
- `NUMBER` is 2–15 decimal digits and must be unique across both native tables.
- `111` is reserved for the local voicemail application; `112` and `911` are
  reserved emergency codes and cannot be bound as subscriber numbers.
- Both Asterisk records update in one SQLite transaction. A conflicting number
  returns HTTP `409`; a missing registration returns `404`.
- The response includes `binding_consistent` and `tmsi_projection`. TMSI update
  is a best-effort projection: `updated|not_present|unavailable|failed`.

号码在两个 Asterisk 原生表中事务更新并保持唯一。TMSI 仅为可选投影；其更新失败不撤销
权威绑定，也不应被解释为签约失败。

This lab dialplan provides no PSTN interconnection and makes no real emergency
calling guarantee. Reserved-code handling only prevents accidental subscriber
assignment. 本实验系统不接入真实 PSTN，也不保证真实紧急呼叫；保留号码仅用于防误绑定。

## Unbind a number / 解绑号码

```http
DELETE /api/v1/subscribers/IMSI/number
```

Unbinding clears the number/routing fields transactionally. It does not delete
the subscriber, rewrite the SIM, or imply that a currently attached device has
detached. 解绑仅清除号码路由，不删除签约、不写 SIM、也不代表终端已离网。

## Safe workflow / 安全流程

1. Back up `/data/state/asterisk/sqlite3.db` with SQLite `.backup`.
2. Use a dedicated placeholder/test IMSI and a number absent from the registry.
3. Compare `GET /subscribers/{imsi}` before and after `PUT`.
4. Treat `GET /connections` as observation only.
5. Exercise voice/SMS and verify delivery separately.
6. Use `DELETE .../number` to remove the test binding when complete.

Postman defaults to `enable_mutations=false`, so bind/unbind requests are skipped
until the operator explicitly opens a controlled mutation window. Postman 默认不执行绑定/解绑。
