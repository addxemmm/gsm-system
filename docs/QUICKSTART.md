# Quick start 2.1 / 2.1 快速开始

> Run Docker, SDR, and RF commands only on the SDR host. The 2.1 management
> plane is deployed and non-RF acceptance passed. RF and handset acceptance are
> still pending. Docker、SDR 与射频命令仅在 SDR 服务器执行；2.1 管理面已部署并通过
> 非射频验收，射频与真机验收仍待完成。

Prerequisites / 前置条件：Ubuntu 22.04, Docker Compose, a USB3-connected B210,
port `8082`, the external `docker_gsm-data` volume, and externally provisioned
test SIMs. Replace every placeholder; never commit real IMSIs, numbers, tokens,
or host addresses. 需准备 B210、Docker、外部数据卷与已在外部制卡的测试 SIM。

## 1. Deploy without transmitting / 部署但不发射

Set repository-root `.env` as described in [DEPLOY.md](DEPLOY.md): blank
`GSM_API_TOKEN` disables authentication; a non-empty value enables it.
按 DEPLOY.md 配置项目根 `.env`：`GSM_API_TOKEN` 留空关闭鉴权，非空启用。

```bash
cd ~/gsm-system
docker volume inspect docker_gsm-data >/dev/null || docker volume create docker_gsm-data
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live
```

Container creation does not start a cell. First inspect non-RF state:

```bash
BASE=http://127.0.0.1:8082/api/v1
AUTH="" # or: AUTH="Authorization: Bearer TOKEN"
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/health"; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/cell"; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/profile"; echo
```

`health.data.ok=true` means the HTTP process is healthy. `cell.data.ready=true`
means all five managed processes are alive; neither value proves RF, attach,
SMS delivery, or voice quality. 健康与进程就绪均不等于射频/真机验收。

After every container recreation, while the cell is still stopped, inspect and
idempotently apply the runtime NAT rule to the container bridge interface:

```bash
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/network?iface=eth0"; echo
curl -fsS -X PUT "$BASE/network" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' -d '{"iface":"eth0"}'
```

`persisted:false` is expected: repeat this stopped-state `PUT` after each
recreation/firewall reset before any preset or explicit cell start. 容器重建后、
任何预设或显式启动前，必须在小区停止态对 `eth0` 执行此幂等 PUT。

## 2. Start one legal test cell / 启动合法测试小区

Confirm the antenna, permitted band/channel, RF kill path, and container
interface. With the default Compose bridge, required `network` is `eth0` (not a
host NIC such as `ens33`). The first start can use a complete profile:

```bash
curl -fsS -X POST "$BASE/cell" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' \
  -d '{"arfcns":"1","c0":"C0","band":"1800","mcc":"MCC","mnc":"MNC","lac":"LAC","ci":"CI","short_name":"LAB","network":"eth0"}'
```

- `arfcns` is exactly `"1"`.
- GSM 900 `c0`: `0..124` or `975..1023`; DCS 1800: `512..885`.
- `lac`: `1..65279`; `ci`: `0..65535`.
- Or create a user preset while stopped, then start it explicitly:

```bash
curl -fsS -X POST "$BASE/presets" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' \
  -d '{"id":"lab-900","name":"Lab 900","description":"Indoor fixture","params":{"arfcns":"1","c0":"55","band":"900","mcc":"001","mnc":"01","lac":"1","ci":"1","short_name":"LAB","network":"eth0"}}'
curl -fsS -X POST "$BASE/cell" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' -d '{"preset_id":"lab-900"}'
```

  Preset CRUD alone never starts/reconfigures the cell; only the second command
  above starts it. A new data volume has no built-in/operator presets. / 预设 CRUD
  本身不启动小区，仅第二条命令启动；新数据卷无内置预设。
- A later `POST /cell` with `{}` reuses the saved last profile.
  后续 `{}` 仅复用已有有效的上次存档。

Poll `GET /cell`; expect `state=running`, `ready=true`, and inspect
`sms_ready`/`voice_ready` separately.

## 3. Connections versus subscribers / 连接与签约

After manual network selection and attach:

```bash
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/connections"; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/subscribers"; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/subscribers/IMSI"; echo
```

`connections` is a volatile observation and may be empty; it is not proof of
offline/online subscriber status. `subscribers` is the persistent Asterisk
registry. `/connections` 是当前观察视图，不能当作持久签约或在线判定。

Bind a unique test number only inside a controlled mutation window:

```bash
curl -fsS -X PUT "$BASE/subscribers/IMSI/number" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' -d '{"number":"NUMBER"}'
```

The Asterisk update is transactional. `tmsi_projection` is best effort and may
be `updated`, `not_present`, `unavailable`, or `failed`; it does not roll back the
authoritative binding. Asterisk 更新是事务性的，TMSI 投影为可选缓存更新。

## 4. SMS and voice / 短信与语音

```bash
curl -fsS -X POST "$BASE/sms" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' \
  -d '{"imsi":"IMSI","sender":"NUMBER","text":"hello"}'
# HTTP 202, data.status == "submitted"

curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/sms"; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/calls"; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/calls/history"; echo
```

SMS accepts at most 159 bytes from the documented safe ASCII/GSM-default-basic
intersection. It rejects UCS-2/Chinese, extension-table characters, quotes, and
backticks. HTTP `202` means submitted directly through the OpenBTS CLI, not
delivered and not queued through an API-owned `smqueue` job. 短信 `202` 只表示提交，
不表示送达。

`GET /calls` lists current Asterisk channels. `/calls/history` reads real
Asterisk CSV CDR rows and normalizes timestamps to UTC; neither endpoint creates
synthetic calls. 语音状态与历史均来自 Asterisk 真实数据。

## 5. Network and stop / 网络与停止

```bash
curl -fsS -X DELETE "$BASE/cell" ${AUTH:+-H "$AUTH"}; echo
curl -fsS ${AUTH:+-H "$AUTH"} "$BASE/network?iface=eth0"; echo
curl -fsS -X PUT "$BASE/network" ${AUTH:+-H "$AUTH"} \
  -H 'Content-Type: application/json' -d '{"iface":"eth0"}'
```

Stop is idempotent and sends TERM before KILL, allowing Asterisk time to flush
CDR data. Only after stop, network `PUT` is idempotent and valid;
`persisted:false` means
the managed iptables rule must be reapplied after a host/firewall reset.
`ipv4_forwarding` is read-only (`true|false|null`); the API does not change
sysctl, and `rule_present` alone is not proof of forwarding. 运行中小区必须先
DELETE 停止，再执行网络 GET/PUT；否则 PUT 返回 `409`。

## Troubleshooting / 故障排查

| Symptom / 现象 | Check / 检查 |
|---|---|
| `40101` | configure the matching Bearer `TOKEN`, or omit auth when the server token is empty |
| `40901` | stop the cell before config/network mutation; do not start twice |
| `41201` | inspect `sms_ready`, `voice_ready`, or required native process state |
| `42201` | inspect `data.errors`; verify radio ranges, IMSI, number, SMS alphabet, and interface |
| `50301` | inspect the named native dependency on the SDR host; do not infer RF success from health |
| empty connections | wait for/manual-select the test network; compare the persistent subscriber record |
| SMS not received | `202` is submission only; inspect `/data/log/smqueue.log` and handset state |
| no CDR | verify `/data/log/asterisk/cdr-csv/Master.csv`, CDR modules, and a completed call |

Use the Postman example environment with `enable_mutations=false` for default
read-only inspection. Preset CRUD requires `enable_mutations=true`; cell-start
requests additionally require the explicit `enable_rf_start=true` switch and
have no automated RF assertions. 预设 CRUD 需开启 `enable_mutations`；启动小区
还需显式开启 `enable_rf_start`，且不包含自动 RF 验收。

For a management-plane-only preset CRUD/error smoke test (no valid cell start
and no RF), run from the development checkout. The script reads the optional
token from `GSM_API_TOKEN` and cleans up its unique fixture in `finally`:

```powershell
.\scripts\tests\presets_smoke.ps1 -BaseUrl http://HOST:8082/api/v1
```

该脚本仅验证预设 CRUD、`409/404/422`，不发送有效小区启动，可从
`GSM_API_TOKEN` 读取令牌，并在 `finally` 中清理唯一测试预设。

On the Ubuntu SDR server only, an already-built image can be checked without
RF hardware or host exposure:

```bash
./scripts/tests/test_presets_image.sh IMAGE
```

The script uses an isolated temporary volume, publishes no port, grants no USB
or privileged access, and mocks the UHD detector with `/bin/false`. It covers
optional-description CRUD, restart persistence, and both preset/explicit start
paths ending at expected `503 no hardware`, then cleans up. 此 Docker 脚本仅在
Ubuntu SDR 服务器执行；使用独立临时卷、不映射端口、不授予 USB/特权，
两种启动路径都在无硬件阶段停止，不发射。
