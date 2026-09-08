# Operations and troubleshooting 2.1 / 2.1 操作与排障手册

This is a workflow, not a live health report. For dated deployment evidence see
[RELEASE-2.1.md](RELEASE-2.1.md); for exact payloads see [API.md](API.md).
本页是操作流程，不是实时状态报告。部署证据查看发布记录，完整字段查看 API 契约。

## 1. Startup settings / 启动配置

| Setting / 配置 | Meaning / 含义 |
|---|---|
| `GSM_API_TOKEN=` | Empty disables Bearer auth; nonempty enables it / 留空免鉴权，非空启用 |
| `TZ=Asia/Shanghai` | Default UTC+08:00; another installed IANA zone is supported / 默认东八区，可自定义 IANA 时区 |
| `GSM_BIND_ADDRESS` | Host address publishing TCP 8082, preferably the intended LAN interface / 管理端口宿主机绑定地址 |
| `GSM_BRIDGE_SUBNET` | Must not overlap LAN/VPN/LTE or handset GPRS pool / 不与其他网络或手机地址池重叠 |
| profile `network=eth0` | Container uplink, not the host NIC / 容器网卡，不是宿主机网卡 |

Keep real values in the ignored root `.env` and private Postman environment.
Do not commit tokens, SIM keys or actual subscriber data. Use the gated deployment
flow in [DEPLOY.md](DEPLOY.md) for environment changes; changing a Postman variable
does not change the server configuration. Timezone controls display, not the
kernel clock. CDR storage remains UTC and API display uses the project zone.
真实值留在被 Git 忽略的根 `.env` 与私有 Postman 环境，不提交令牌、SIM 密钥或签约数据。
服务端环境变化走部署门禁；修改 Postman 变量不等于修改服务器配置。时区只改变展示，
不手动给系统时钟加八小时；话单原始存储为 UTC，接口按项目时区展示。

## 2. Daily sequence / 日常操作顺序

1. Read `GET /health`, `GET /cell`, `GET /profile` and `GET /presets`.
   / 先查管理面、小区、存档与预设，不凭容器 Up 判定业务正常。
2. After container recreation/firewall reset, while stopped, call
   `PUT /network` with `{"iface":"eth0"}`. Check `rule_present` and read-only
   `ipv4_forwarding`; repeat PUT should return `changed:false`.
   / 重建或防火墙重置后，在停止态配置出口 NAT 并复查；规则存在不等于转发已开启。
3. Choose one `POST /cell` form: `{"preset_id":"0"}`, a complete custom
   profile, or `{}` for an existing valid saved profile. Never mix forms.
   Preset CRUD saves configuration without starting RF.
   / 预设、自定义、已有存档复用三选一，不混合字段；预设增删改本身不启动小区。
4. Inspect `GET /cell` for `running/ready`, then reattach the test handsets.
   Compare `/connections` observations with persistent `/subscribers` bindings.
   / 就绪后手机重新接入，对照连接观察与持久绑定；连接行不等于在线设备。
5. Validate service in layers: 2600 → 2602 → peer calls; Chinese handset SMS
   in both directions → ASCII API submission; GPRS separately.
   / 分层验收测试号码、互拨、双向中文短信、API 下发；数据上网单独检查。
6. Use **`DELETE /api/v1/cell`** to stop. Verify `state=stopped` and all native
   process flags false before network/configuration changes or deployment.
   / 停止接口幂等；操作后查状态，再进行网络、配置或发布变更。

## 3. Status is not delivery / 状态与业务结果分开判断

| Evidence / 证据 | Proves / 说明 | Does not prove / 不代表 |
|---|---|---|
| `/health` HTTP 200, `ok:true` | Management handler responded / 管理面响应 | Radio or handset health / 射频或真机健康 |
| `/cell state=stopped` | Native cell services stopped / 小区服务已停止 | Container failure / 容器故障 |
| `/cell state=running, ready=true` | Five managed processes alive / 五进程存活 | Call quality, SMS receipt, Internet / 通话质量、送达、上网 |
| `/cell state=degraded` | Incomplete native service set / 原生进程组合不完整 | A unique root cause / 唯一故障原因 |
| Docker `healthy` | State-aware management probe passed / 管理状态探针通过 | Sustained radio stability / 长时间无线稳定 |
| SMS HTTP 202 | OpenBTS CLI accepted submission / CLI 接受提交 | Delivery receipt / 送达回执 |
| `/connections ip:null` | No packet-data IP observed / 未观察到数据 IP | SMS failure or proven NAT fault / 短信故障或已证实的 NAT 故障 |

Postman contract tests can pass on a well-formed degraded snapshot. Always read
the state and warning; a green schema test is not a deployment health gate.
Postman 对格式正确的降级状态可以通过契约测试；仍须查看状态和提示，不将绿色断言
当作部署健康验收。详见 [Postman README](../postman/README.md)。

## 4. SMS records, identity and latency / 短信记录、身份与时延

- **Chinese:** new native observations support BMP UCS-2 Chinese; API outbound
  remains safe ASCII, at most 159 bytes. See [SMS-UNICODE.md](SMS-UNICODE.md) for
  unsupported DCS, malformed data, emoji/surrogate and multipart limitations.
  / 中文接收记录与 API 发送是两条路径，不能用接收能力推断 API 已支持中文发送。
- **Scope:** `scope=current_start` selects observations in the current known cell
  start window. Stop freezes it; a new accepted start resets it; manager recreation
  starts with `session:null`. Parameter rejection or duplicate-start 409 does not
  erase the window. Raw historical logs remain, and old queued messages processed
  now may appear as current observations.
  / 范围是本轮观察时刻，不是短信最初创建时刻；管理进程重建后空列表不等于数据删除。
- **Null identities:** service code 101/411/2600/2602 has no handset IMSI. Missing
  identities are filled only from a unique, consistent current binding; provenance
  distinguishes observed values from query-time lookup. Unknown stays null.
  / 服务号码不伪造 IMSI；唯一且一致的当前绑定才能补全，来源字段区分日志与查询时关联。
- **Deduplication:** identity/qtag within a process generation merges repeated
  observations. Different messages with identical text remain separate records.
  Multipart segments are not joined. / 按消息身份去重，不按正文去重；片段不重组。
- **Delay:** fast 101 replies do not prove another handset's paging works. Pinned
  smqueue can wait 15 seconds for acknowledgement then 60 seconds before retry,
  while some lookup errors wait 300 seconds. These are possible amplification
  paths, not a measured explanation for every slow SMS. Do not merely change the
  read-but-unused `SIP.Timeout.MessageResend` setting and claim improvement.
  / 101 快不代表另一手机的下行寻呼正常；先关联具体事务，再决定是否调整重试。

For a reproducible test, use two private test devices named A/B in the report,
distinct short text per attempt, one message at a time. Record send/receive times,
API request_id and current session ID. Compare A→B, B→A and API→A/B separately.
Record missing receipts explicitly; observation presence is not a receipt. Share
redacted metadata, not actual IMSIs, SIM keys or private message bodies.
复现时报告只用 A/B 区分手机，一次一条不同短消息，记录发送/接收时刻、request_id 与
session ID；分开对比三个方向，未收到就明确记录，不用日志存在代替送达证据。

## 5. Calls and test numbers / 通话与测试号码

| Order / 顺序 | Test / 测试 | Expected / 预期 |
|---|---|---|
| 1 | 2600 | Echo, max 60 seconds, # exits / 回声，最长 60 秒 |
| 2 | 2602 | Test tone, max 30 seconds / 测试音，最长 30 秒 |
| 3 | A→B and B→A | Ringing, correct bound caller number, two-way audio / 振铃、正确绑定来电号、双向声音 |
| 4 | `/calls/history` | Actual CDR disposition and timing / 实际话单结果与时间 |

The service numbers are built in and reserved, not subscriber bindings. If both
diagnostics fail, inspect native process state, TCH assignment and SIP outcomes
before blaming peer-number routing. A SIP cause 3/no-route problem differs from
observed SIP 502/cause 27, L2 errors or a channel still awaiting release. For
caller-number issues compare the current binding with CDR and Asterisk routing;
do not substitute an arbitrary caller ID.
测试号码无需绑定。两者都失败时先看进程、语音信道分配与 SIP；缺路由与无线/信道异常
分开定位。来电号码异常须对照绑定、话单和 Asterisk 路由，不填造假的固定主叫号码。
See [VOICE-DIAGNOSTICS.md](VOICE-DIAGNOSTICS.md) / 详见语音诊断。

### Packet data is a separate path / 分组数据单独验收

Check in order: GPRS enabled and handset data/roaming settings → SGSN/PDP
activation and observed handset IP → packet traffic through the container TUN
and forwarding/NAT → reachable upstream DNS → handset HTTP/HTTPS. An allocated
IP or `rule_present:true` alone is insufficient. New volumes seed GPRS enabled;
recreation preserves existing database settings, including explicit disablement.
按顺序检查 GPRS/手机数据设置、SGSN/PDP 与手机 IP、TUN 流量与转发/NAT、上游 DNS、
手机网页访问。新卷默认启用不等于旧卷设置被覆盖；仅有 IP 或 NAT 规则不代表上网成功。
Do not tune GPRS channel allocation solely from an SMS/voice delay report.
短信/通话慢也不是直接修改 GPRS 信道分配的依据。详见 [SIM.md](SIM.md)。

## 6. Degraded and evidence collection / 降级与证据采集

Run the following **read-only** checks on the SDR server, not the development
machine. Read logs before replacing a container. Log output may contain private
identifiers and SMS content; inspect locally and redact before sharing.
以下只读检查仅在 SDR 服务器执行；替换容器前保留证据，输出可能含隐私，分享前脱敏。

```bash
date -Is
docker inspect --format '{{.Config.Image}} {{.Image}} {{.State.Status}} {{.State.Health.Status}}' gsmsystem-uhd4
docker exec gsmsystem-uhd4 /usr/local/bin/gsm-system --version
docker exec gsmsystem-uhd4 /usr/local/bin/gsm-system --healthcheck
docker exec gsmsystem-uhd4 date -Is
docker logs --tail 150 gsmsystem-uhd4
docker exec gsmsystem-uhd4 tail -n 150 /data/log/openbts.log
docker exec gsmsystem-uhd4 tail -n 150 /data/log/openbts-syslog.log
docker exec gsmsystem-uhd4 tail -n 150 /data/log/smqueue.log
journalctl -k --since '-15 min' --no-pager
```

`--healthcheck` deliberately exits nonzero for degraded/transitioning states;
continue collecting the other evidence. Kernel logs may require the server's
normal elevated log-reading permission. Do not run a competing UHD device scan
while the transceiver owns the radio.
降级/切换态探针退出非零是预期结果，继续收集其余证据；内核日志可能需要常规读取权限。
transceiver 占用设备时不启动竞争性的 UHD 探测。

Look for `Receive recovered after`, `Receive timeout persisted`, or
`Discontinuous receive after timeout`. The patch tolerates only bounded transient
waits with continuous samples. Real sample loss or persistent USB/FPGA trouble
still stops service; helper processes may remain, resulting in degraded.
Do not repeatedly restart RF to hide this state. After inspection, explicitly
stop the cell and verify stopped before an operator-chosen new start.
记录恢复、耗尽预算、时间戳不连续等日志；有限重试不掩盖真实丢样或持续硬件故障。
排查后显式停止并确认，再由操作者决定是否重新启动，不用自动反复开射频掩盖故障。
See [UHD-RX-RECOVERY.md](UHD-RX-RECOVERY.md) / 详见有限恢复边界。

## 7. Release and cleanup checklist / 发布与清理核对表

- Runtime tag stays `gsm-system:2.1`; implementation identity comes from OCI and
  binary revision. A newer documentation commit does not mean an older runtime
  implementation was accidentally deployed. / 标签固定，文档提交与运行实现版本分开。
- Follow [DEPLOY.md](DEPLOY.md), run all four isolated image suites for code/image
  changes, then verify the new container before cleanup. / 代码/镜像变更验收后发布清理。
- Cleanup removes obsolete GSM runtime objects and unused build cache, not LTE,
  business volumes, bindings or SMS/CDR history. Go/Ubuntu build bases may remain.
  There is no retained rollback container/image. / 清理非业务数据，不保留旧 GSM 回滚镜像。
- Documentation/Postman-only changes require offline checks and commit/push,
  not a rebuild, RF restart or a new runtime revision marker.
  / 纯文档与 Postman 调整执行离线检查和提交，不重建镜像、不重启射频、不改发布标记。

```bash
docker ps -a --format '{{.Names}} {{.Image}} {{.Status}}'
docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}'
docker volume ls
docker system df
```

These inventory commands delete nothing. Avoid broad `docker system prune` or
volume pruning; use the release script's scoped cleanup after successful gates.
以上清单命令不删除数据；不要全局清理卷或系统对象，由发布脚本在验收成功后按范围清理。
