# Rules 使用规则 (MUST READ 必读)

## 1. What stateless means 无状态含义

This tool has **no accounts, no background jobs, no Go-layer database**:
本工具**无账号、无后台任务、Go层无数据库**。State lives only in three places
状态只在三处：

| State 状态 | Location 位置 | Lifetime 生命周期 |
|---|---|---|
| Last launch params 上次启动参数 (arfcns/c0/band/mcc/mnc/lac/ci/short_name/network) | `/data/last_start.json` (volume `gsm-data`) | survives recreates 重建不丢 |
| Seeds 种子 (smqueue seed SQL, OpenBTS dumps) | image `/app/configs/` | follow image 跟随镜像 |
| Runtime 运行态 (OpenBTS/transceiver/sipauthserve/smqueue/asterisk, attached UE, smqueue.log) | memory + `/data/log/` | `/stop` or recreate clears 清零 |

In short 简言之：**configure once, reuse; stop means fully stopped, no ghost processes 配一次、复用；停即全停、无幽灵进程。**

smqueue queue DB starts clean every time (documented legacy v1.3 behavior):
smqueue队列库每次启动都是干净的（沿用v1.3行为并文档化）。SMS history is read
from `smqueue.log`, not the DB; persistence would only risk duplicates/corruption
after `kill -9`. 短信历史读日志不读库，持久化只会带来重复/损坏风险。

## 2. Standard flow 标准操作流

```bash
# Boot 开机 (once per power-on, then reuse):
curl -X POST http://127.0.0.1:8082/api/v1/cell -H 'Content-Type: application/json' -d '{}'
#   ^ empty = reuse last profile 空body=复用存档. First boot without profile → 422.
#   Partial override 部分覆盖亦可：-d '{"short_name":"lab"}'

# Inspect 查看 / archive 存档
curl http://127.0.0.1:8082/api/v1/cell
curl http://127.0.0.1:8082/api/v1/profile

# Shutdown 关机
curl -X DELETE http://127.0.0.1:8082/api/v1/cell
```

- One cell at a time; start while running → `409`. Stop first.
- `/stop` kills the 5 processes and removes our NAT rule; other host rules untouched.
- Recreate never auto-transmits (compliance); manual start with `{}` restores.

## 3. Config precedence 配置优先级

`/start` params (highest 最高) → saved `last_start.json` (inherit 空字段继承) →
server defaults `configs/app.yaml.example`.

## 4. Subscribers & SIM 签约与SIM卡

- No SIM-writer API: SIMs are written externally. 无写卡接口，卡在外部写好。
- `TMSITable.db` is volatile runtime (`tmsis clear` on start/stop); the persistent
  registry is `sqlite3.db` (`sip_buddies`/`dialdata_table`). Use explicit
  `POST /api/v1/subscribers` to set numbers — no implicit behavior.
- After UE attaches, set its number, then voice/SMS between two UEs works via Asterisk.

## 5. Upgrade & rollback 升级与回滚

```bash
cd ~/gsm-system && git pull
sudo docker compose -f deploy/docker/docker-compose.yml up -d --build
# Rollback 回滚：old image gsmsystem-dep:1.3 + container gsmsystem keeps on host.
```

`/data` volume (profile/logs) survives upgrades.

## 6. RF discipline 射频纪律

- Confirm band/antenna (TX/RX connected), keep distance before transmitting.
- B210 over VM USB: defaults are conservative; watch `openbts.log` for underruns.
- Never leave the cell transmitting unattended: stop after tests.
