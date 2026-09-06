# Rules 使用规则 (MUST READ 必读)

## 1. What stateless means 无状态含义

This tool has **no accounts, no background jobs, no Go-layer database**:
本工具**无账号、无后台任务、Go层无数据库**。State locations
状态存放位置：

| State 状态 | Location 位置 | Lifetime 生命周期 |
|---|---|---|
| Last launch params 上次启动参数 (arfcns/c0/band/mcc/mnc/lac/ci/short_name/network) | `/data/last_start.json` (volume `docker_gsm-data`) | survives recreates 重建不丢 |
| Seeds 种子 (OpenBTS example schema, clean-DB inits) | image `/app/seeds/` + `/OpenBTS/*_init.db` | follow image 跟随镜像 |
| Native configuration and subscriber registry 原生配置及签约库 | `/data/state/OpenBTS/` + `/data/state/asterisk/` | survives restart/recreate 重启与重建保留 |
| Runtime 运行态 (OpenBTS/transceiver/sipauthserve/smqueue/asterisk, attached UE, smqueue.log) | memory + `/data/log/` | `/stop` or recreate clears 清零 |

In short 简言之：**configure once, reuse; stop means fully stopped, no ghost processes 配一次、复用；停即全停、无幽灵进程。**

TMSI is reset at container boot; persistent subscriber/configuration databases
are initialized only when absent. Do not treat `/etc/OpenBTS/smqueue.db` (service
configuration) as an SMS history database. SMS history endpoints read
`/data/log/smqueue.log`; a working log pipeline is required independently.
容器启动时重置 TMSI；持久签约及配置库仅缺失时初始化。`smqueue.db` 是服务
配置库，不是短信历史；短信历史接口读取 `/data/log/smqueue.log`，需另行确保日志链路有效。

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
- `/stop` kills the 5 processes. NAT rules are NOT managed by start/stop:
  they are added only by explicit `POST /iptables` or `/api/v1/network`
  calls and persist until you delete them (same as legacy behavior).
- Recreate never auto-transmits (compliance); manual start with `{}` restores.

## 3. Config precedence 配置优先级

`/start` params (highest 最高) → saved `last_start.json` (empty fields
inherit 空字段继承). There are no other server-side defaults on the v1
path: missing required fields → `422` (legacy bare `POST /start` without
any profile falls back to the id=0 preset instead — frozen behavior).

## 4. Subscribers & SIM 签约与SIM卡

- No SIM-writer API: SIMs are written externally. 无写卡接口，卡在外部写好。
- `TMSITable.db` is volatile runtime (`tmsis clear` on start/stop); the persistent
  registry is `/data/state/asterisk/sqlite3.db` (`sip_buddies`/`dialdata_table`),
  exposed through `/var/lib/asterisk/sqlite3dir`. Use explicit
  `POST /api/v1/subscribers` to set numbers — no implicit behavior.
- After UE attaches, set its number, then voice/SMS between two UEs works via Asterisk.

## 5. Upgrade & rollback 升级与回滚

```bash
cd ~/gsm-system
# First migration: back up native databases before replacing the old container.
# 首次迁移：先备份原生数据库，再替换旧容器，详细步骤见 DEPLOY.md。
docker compose -p gsm-system-live -f deploy/docker/docker-compose.uhd4.yml build
```

See [DEPLOY](DEPLOY.md) for migration, cutover and rollback. Keep a same-UHD4
rollback image/container; the Xenial image is not a working clone-board fallback.
迁移、切换及回滚见部署文档。保留同 UHD4 版本的旧镜像和容器；Xenial 镜像不适用于该克隆板回滚。

`/data` volume (profile/logs/native databases) survives upgrades. Do not use
`docker compose down -v` unless intentional data deletion is desired.
`/data` 卷保存配置档案、日志及原生数据库；保留卷，不以 `down -v` 作为普通升级操作。

## 6. RF discipline 射频纪律

- Confirm band/antenna (TX/RX connected), keep distance before transmitting.
- B210 over VM USB: defaults are conservative; watch `openbts.log` for underruns.
- Never leave the cell transmitting unattended: stop after tests.
