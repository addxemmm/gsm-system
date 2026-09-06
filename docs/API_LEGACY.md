# GSM-System Legacy API Reference 旧版接口参考（根路径，已冻结 Frozen）

> **New integrations use 新集成请用 [`/api/v1`](API.md).**
> This file records the frozen legacy routes verbatim 逐字记录冻结旧路由：
> behavior never changes (critical bugfix only), `message_id` semantics kept forever.
> Implementation 实现：`internal/api/server.go`. `message` wording, `message_id`,
> field names follow this doc; changing impl must sync this doc + `*_test.go`.

- Base 基地址：`http://<server>:8082` (host `http://127.0.0.1:8082`)
- 10 legacy routes, all `POST` + JSON. Plus `GET /healthz`, `GET /status`, `GET /profile`.
- **HTTP status always 200 恒为200** (success or failure); check JSON `status`.
- Envelope 包络：`{"status": bool, "message_id": int, "message": str, ...}`.
- `message_id = 0` everywhere = generic failure, inspect logs
  (`docker logs gsmsystem-uhd4` / `/data/log/`).
- Non-POST to a POST route returns that route's own `message_id 0` text.

## Compatibility Matrix 兼容矩阵

Source of truth 以 `gsmsystem/run.py` 为准 (`run_c.py`/README differences noted).

| # | Route 路由 | Request 请求 | Response 响应原文 | Backend 后端依赖 |
|---|---|---|---|---|
| 1 | `POST /start` | no args 无参 | `0 Start Failed / 1 Start successfully / 2 is running / 3 device is not connected, please connect usrp device.` | `ps -eo pid,stat,comm` OpenBTS check → `rm /var/run/OpenBTS.pid` → `uhd_find_devices\|grep B210` → start sipauthserve/smqueue/asterisk/OpenBTS → `OpenBTSCLI -c tmsis clear` |
| 2 | `POST /stop` | no args | `0 Stop failed. / 1 Stop successfully. / 2 Not running.` | check 5 procs (OpenBTS/sipauthserve/smqueue/asterisk/transceiver) → `kill TERM→KILL` + `tmsis clear` + `rm pid` |
| 3 | `POST /config` | `{"id":0-4}` | `0 Failed. / 1 Success, please start system manually. / 2 Stop failed, please stop manually. / 3 Can not find database file. / 4 The config id is not existed.` | stop cell first, then `sqlite3 /etc/OpenBTS/OpenBTS.db UPDATE CONFIG` 8 keys (ARFCNs/C0/Band/MCC/MNC/LAC/CI/ShortName). Presets see `internal/gsm/gsm.go:Presets` |
| 4 | `POST /getconfig` | no args | `+ data:[[KEY,VALUE]...]` `0 Failed / 1 Success / 3 Can not find database file.` | `SELECT KEYSTRING,VALUESTRING FROM CONFIG` |
| 5 | `POST /allconfig` | `{"name":"GSM.Identity.ShortName","value":"gsmsystem"}` | `0 Failed / 1 Success / 2 Stop failed or No db` | stop first, single `UPDATE CONFIG`. Legacy allowed arbitrary keys (kept compat, shape-checked in Go) |
| 6 | `POST /iptables` | `{"iface":"eth0"}` (legacy example `wlo1`) | `0 Failed / 1 Success` | `iptables -t nat -A POSTROUTING -s 192.168.99.0/24 -o $iface -j MASQUERADE` (argv exec, iface metachar-rejected) |
| 7 | `POST /smsinfo` | no args | `+ infos:[[time,sms,from_num,from_imsi,to_num,to_imsi]]` `0 Failed / 1 Success / 2 Can not find /var/log/smqueue.log / 3 SMS message not fount.` (typo kept) | parse `/data/log/smqueue.log` (`get_text: Decoded text` + `Deliver message:` 13-line SIP block). Code reads smqueue.log; README `syslog` is stale. Chinese SMS not retrievable |
| 8 | `POST /ueinfo` | no args | `+ infos:[[imsi,imei,number,ip]]` `0 Failed / 1 Success / 2 System is not running. / 3 Can not find info.` | `OpenBTSCLI -c sgsn list` (IMSI+IP) join `OpenBTSCLI -c tmsis -l` (imsi/imei/number). `none`/empty IP = no PDP address |
| 9 | `POST /setphonenumber` | `{"imsi":"00101...","number":"10000002"}` | `0 False / 1 Success / 2 Can not find TMSITable/asterisk db / 3 imsi not existed / 4 not existed in asterisk` | `sqlite3 /var/run/TMSITable.db:tmsi_table(IMSI→ASSOCIATED_URI)` + `/var/lib/asterisk/sqlite3dir/sqlite3.db:sip_buddies(callerid)+dialdata_table(exten)` |
| 10 | `POST /sendsms` | `{"imsi":"...","sender":"1111111","smsmessage":"hello"}` | `0 Send failed. / 1 Send successfully. / 2 Not running.` | `OpenBTSCLI -c sendsms <imsi> <sender> <msg>`, success iff output contains `message submitted for delivery`. ASCII only (Chinese unsupported) |

## Known drifts 已知漂移（以代码为准）

- `smsinfo id=2` path: code `/var/log/smqueue.log` (Go: `/data/log/smqueue.log`), README `syslog` stale.
- `sendsms` table in old README copied `stop` wording; code wording above wins.
- `run_c.py` lacks `tmsis clear` on start, lacks `id` bounds check (Go keeps `run.py` semantics).
- `config`/`allconfig` stop the cell first (documented behavior, kept).

## Typical flow 典型流程

```bash
BASE=http://127.0.0.1:8082
curl -s -X POST $BASE/stop -d '{}'
curl -s -X POST $BASE/start -d '{}'
curl -s -X POST $BASE/ueinfo -d '{}'
curl -s -X POST $BASE/smsinfo -d '{}'
curl -s -X POST $BASE/sendsms -H 'Content-Type: application/json' -d '{"imsi":"001010123456780","sender":"101","smsmessage":"hello"}'
```
