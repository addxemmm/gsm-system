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

### Interpret registration evidence / 解读注册证据

TMSI/SGSN rows are observations of native registration activity, not an
admitted or online-device list. The additive `auth` and `reject_code` connection
fields are optional during a rolling 2.1 upgrade and are nullable integers when
present. They preserve native recorded values rather than calculating a new
status: `0=unauthorized`, `1=registrar-authorized`, `2=open-registration`, and
`3=fail-open`. None is a real-time online guarantee. `reject_code:4` has multiple
possible registration-failure causes and does not alone prove a Ki mismatch.

TMSI/SGSN 行是原生注册活动的观察值，不是已入网或在线名单。滚动升级期间新增的
`auth`、`reject_code` 可缺省；存在时为可空整数并按原生值透传。AUTH 值为
`0=未授权`、`1=Registrar 授权`、`2=开放注册`、`3=失败时开放`，均非实时在线
证明；拒绝码 4 可能有多种原因，
不能据此单独断定 Ki 错误。

A synthetic, redacted diagnostic pattern may look like this:

```text
GPRS.Enable=0
SGSN.IPs=
connections: 3 synthetic source rows; each has native TMSI absent, number=null, ip=null, auth=0, reject_code=4
SIP_BUDDIES rows=0; DIALDATA rows=0
sipauthserve: repeated IMSI unknown
network: rule_present=true; ipv4_forwarding=true
```

This pattern shows an observed, rejected registration attempt with no persistent
subscriber rows; it does not show three admitted handsets. `ip:null` is also
compatible with healthy NAT: a handset packet-data address appears only after
GPRS is enabled and PDP/SGSN setup succeeds. SMS uses GSM signalling and does
not require that handset data IP. The parser takes `number` from the fixed
native number column and packet-data IPs from `SGSN.IPs=`; a scheduling token
such as `WELCOME_SENT 1` must not become phone number `1`.

该合成例子表示观察到了被拒绝的注册尝试，且持久签约表为空，并不表示三台手机已
入网。即使 NAT 正常，GPRS 关闭或 PDP/SGSN 未就绪时 `ip` 仍会是 `null`；短信走
GSM 信令，不依赖该手机数据 IP。号码来自固定原生列，`WELCOME_SENT 1` 之类的
调度状态不得被误读为号码 `1`。

The cell PLMN (for example `001/01`) is the broadcast MCC/MNC, not an IMSI-prefix
allowlist. A SIM with a different home PLMN placeholder such as `001/11` may try
manual selection, but it must still satisfy the active admission policy. In the
strict registrar mode that means matching provisioning and authentication;
explicit open-registration rules are a different policy, and an empty matching
rule does not enable them. Automatic selection is not restricted to an exact
IMSI-prefix match and may select a permitted roaming network. See 3GPP/ETSI TS
23.122, automatic and manual network selection, clauses
[4.4.3.1.1 and 4.4.3.1.2](https://www.etsi.org/deliver/etsi_ts/123100_123199/123122/18.11.00_60/ts_123122v181100p.pdf).

小区 `001/01` 是广播 PLMN，不是 IMSI 前缀白名单。归属 PLMN 为 `001/11` 的
测试 SIM 可以手选尝试，但仍须满足当前接入策略：严格 Registrar 模式要求匹配
签约与鉴权；开放注册是另一种须显式配置匹配规则的策略，空规则并未开启它。
自动选网也可按规则选择漫游网，并非只选择与 IMSI 前缀相同的 PLMN。

An empty configured welcome message causes no normal welcome SMS submission.
`WELCOME_SENT` is only a native scheduling marker, not delivery evidence. An API
HTTP `202` likewise means submitted, not delivered; verify handset receipt
separately. / 正常欢迎消息为空就不发送；`WELCOME_SENT` 仅是原生调度标记，
不代表送达。API 的 HTTP `202` 同样仅表示已提交，须另行核验手机收件。

At container startup, the image migrates only the two known legacy normal/open
registration welcome defaults (the old addx IMSI prefix and the upstream test-
network IMSI prefix) to the short `101`/`411` onboarding instruction. Operator-
written text and an intentionally empty value remain unchanged. This does not
clear an existing `WELCOME_SENT` marker or prove handset receipt. / 容器启动时仅
把已知的两种旧默认欢迎文案（旧 addx IMSI 前缀、上游测试网络 IMSI 前缀）迁移为
简短的 `101`/`411` 操作提示；保留管理员自定义文案及留空禁用设置。迁移不会清除
已有 `WELCOME_SENT` 标记，也不代表手机已收到。

This LUR welcome is separate from the `SC.Register.Msg.WelcomeA/B` reply sent
after a successful `101` number registration; those values live in the smqueue
database and are not managed by `GET/PATCH /api/v1/config`. / LUR 欢迎短信与
`101` 号码注册成功后的 `SC.Register.Msg.WelcomeA/B` 回执是两套配置；后者位于
smqueue 数据库，不受 `GET/PATCH /api/v1/config` 管理。

## Reattach after an open-registration deployment / 开放注册部署后重新接入

For the temporary-identity attachment deadlock and automatic NAT repair on
explicit start, see [GPRS-RECOVERY.md](GPRS-RECOVERY.md).
临时身份附着阻塞与显式启动自动补齐 NAT 的修复边界见上述文档。

On this pinned OpenBTS build, the built-in SGSN/GGSN parses but does not select
or reject a route by APN, and allocates IPv4 addresses. Configure the handset:
本项目固定版本的内置 SGSN/GGSN 解析 APN，但不按 APN 选择或拒绝路由，分配 IPv4。
手机可使用以下配置：

| Setting / 设置 | Value / 值 |
|---|---|
| APN | `internet` (a conventional label, not an admission whitelist / 常用名称，不是接入白名单) |
| APN type / 类型 | `default` |
| APN protocol and roaming protocol / 协议及漫游协议 | IPv4 |
| Username/password / 用户名、密码 | empty / 留空 |
| Authentication / 认证 | None / 无 |
| MCC/MNC in the handset APN / 手机 APN 中的 MCC/MNC | keep the SIM-derived values / 保持 SIM 自动值 |
| Mobile data, data roaming / 移动数据、数据漫游 | enabled for this test SIM / 对该测试 SIM 开启 |

This implementation processes IPCP DNS options, not PAP/CHAP credentials. Verify
`GGSN.DNS` points to a handset-reachable upstream DNS server, not the Docker
resolver `127.0.0.11` or host loopback stub. The address must also pass the native
GGSN firewall policy. 当前实现处理 IPCP DNS，不做 PAP/CHAP 凭据校验。DNS 必须是
手机可达且符合 GGSN 防火墙策略的上游服务器，不下发 Docker 或宿主机回环解析地址。
Implementation references / 实现依据：pinned `SGSNGGSN/GPRSL3Messages.cpp`,
`SGSNGGSN/Ggsn.cpp`, `SGSNGGSN/miniggsn.cpp`.

### Fresh-volume packet-data defaults / 新卷分组数据默认值

Only when creating a new OpenBTS database, startup sets `GPRS.Enable=1`,
`GPRS.Channels.Min.C0=2`, `GPRS.Channels.Min.CN=0`, and initializes `GGSN.DNS`
from `GSM_GPRS_DNS` (default `1.1.1.1`). Set that environment value to a reachable
upstream IPv4 DNS server before the first initialization. Loopback and non-unicast
values are rejected. Existing volumes retain their entire GPRS/DNS configuration,
including an intentional `GPRS.Enable=0`; changing the environment later does not
overwrite that database. Enabling these defaults does not prove successful PDP,
DNS or Internet traffic; container forwarding/NAT and handset acceptance are
separate checks.

仅创建全新 OpenBTS 数据库时初始化上述 GPRS 开关、最小信道数与 DNS；首次启动前
可通过 `GSM_GPRS_DNS` 指定实际可达的上游 IPv4 DNS，默认 `1.1.1.1`，拒绝回环及
非单播地址。已有卷完整保留原配置，包括主动关闭 GPRS；后续更改环境变量不覆盖
持久库。默认开启不等于 PDP、DNS 或互联网业务验收完成，仍须核验转发/NAT 和真机。

### Modern GMM attach compatibility / 现代手机 GMM 附着兼容

The image applies `0004-gmm-optional-tv-ies.patch` to the pinned
`SGSNGGSN/GPRSL3Messages.cpp`. The optional-IE parser now consumes the one-octet
`C-`, `D-`, `E-`, and `F-` TV fields without reading a nonexistent TLV length;
it also stops MBMS context status from falling through into classmark parsing.
Truncated TLVs remain errors, with diagnostic metadata instead of a complete
identity-bearing frame. These field formats are specified in
[TS 24.008, table 9.4.1](https://www.etsi.org/deliver/etsi_ts/124000_124099/124008/19.05.00_60/ts_124008v190500p.pdf)
and the compatibility issue was reported in
[upstream PR 9](https://github.com/RangeNetworks/openbts/pull/9).

镜像对固定原生源码应用兼容补丁，正确消费 `C-/D-/E-/F-` 单字节 TV 字段，
避免把下一字段或报文结尾误读为 TLV 长度；同时修正 MBMS 分支误落入 classmark
解析。截断 TLV 仍按错误处理，诊断只记录类型、位置及长度，不记录含身份的完整帧。

The 2026-09-08 read-only inspection found repeated `Premature end of GPRS GMM
message type 1 AttachRequest` errors while GPRS, the TUN route, forwarding and
NAT were already enabled. This source defect matches that failure mode, but the
specific handset IE was not captured; it is not a confirmed explanation for
every observed failure. The builder compiles and runs the actual optional-IE
method in isolation with a checked frame adapter. That regression is not a full
GMM attach or handset Internet test. After deployment, verify a fresh packet
attach, PDP-assigned IP, bidirectional TUN/NAT counters, DNS and handset HTTP.

2026-09-08 只读检查发现反复 AttachRequest 解析截断错误，而 GPRS、TUN 路由、
转发和 NAT 已开启。此源码缺陷符合该错误模式，但尚未捕获本次手机的具体 IE，
故不将全部现场失败归因于此。构建器以有界帧适配器编译运行生产可选 IE 方法；
该隔离回归不等于完整附着或手机上网验收。部署后须重新核验分组附着、PDP 地址、
TUN/NAT 双向计数、DNS 及手机 HTTP。

1. With the cell stopped, verify the persisted registration/GPRS settings and
   apply `PUT /api/v1/network` with `{"iface":"eth0"}` after container recreation.
   小区停止时核对持久配置，容器重建后重新应用容器内 NAT。
2. Start using either the preset request or complete custom parameters, then
   enable 2G on the test phone and manually select the test PLMN if needed.
   使用预设或完整自定义参数启动，在测试手机启用 2G，必要时手选测试网络。
3. Reattach by toggling airplane mode. A newly accepted open-registration
   observation normally has `auth:2`; this is not a continuing online guarantee.
   `ip` remains null until packet attach and PDP activation succeed. 飞行模式开关
   后重新接入；开放注册通常记录 `auth:2`，成功建立 PDP 后才出现数据 IP。
4. Verify the welcome SMS on the handset, then send one manual test:
   在手机检查欢迎短信，再发送一条手动测试短信：

   ```http
   POST /api/v1/sms
   Content-Type: application/json

   {"imsi":"IMSI","sender":"101","text":"hello"}
   ```

   Replace `IMSI` with the owned test SIM's 15-digit identity. Receipt, not `202`,
   is acceptance evidence. Direct IMSI-addressed MT SMS does not require a bound
   receiving number or data IP. 替换为自有测试 SIM 的 15 位 IMSI，以手机实收验收；
   直接 IMSI 下发不要求先绑定收件号码，也不要求手机有数据 IP。
5. An empty TMSI seed after container recreation allows a fresh welcome attempt;
   changing the message alone does not clear `WELCOME_SENT`. Open-registration
   admission does not create persistent subscribers or number routes. 容器重建
   后易失 TMSI 从空种子开始；单改欢迎文字不会清除已调度标记。开放注册不会自动
   创建持久签约或号码路由，语音和号码业务须另行配置及验收。

If OpenBTS/transceiver exits with `UHD: Receive timed out`, stop the cell and
inspect USB passthrough/UHD logs. A healthy management container does not resolve
that radio failure. 若出现 UHD 接收超时退出，停止小区并检查 USB 直通和 UHD 日志；
管理容器健康不代表射频故障已消失。

On 2026-09-08 the operator confirmed handset registration and SMS transmit/receive.
Caller-number display and packet-data/DNS/Internet traffic remain separate live
acceptance items. Management, database, parser and container-health checks alone
do not close them. / 2026-09-08 用户已确认手机接入及短信收发正常；主叫号码显示、
分组数据、DNS 和互联网流量仍须独立真机验收，管理面及解析器测试不等同于业务验收。

## Bind a number / 绑定号码

### From the handset / 通过手机短信绑定

Reply to shortcode **101** with a **7–10 digit** number, for example `10000001`.
This native smqueue onboarding path creates the persistent subscriber/routing
rows. Repeat the same number to request its confirmation again; a different
number on an already-bound SIM does not silently replace the binding. Send a
short ASCII message such as `info` to **411** to query system/number information.
回复 **101**，正文只写希望绑定的 **7–10 位数字**，例如 `10000001`。原生 smqueue
会创建持久签约及号码路由。可重发相同号码获取确认；已绑定后回复另一号码不会自动
覆盖旧号码。向 **411** 发送 `info` 可查询系统及号码信息。改号使用下方 API。

The native shortcode's length policy differs from the API's 2–15 digit update
policy. Number-taken and invalid-input responses are native SMS replies, not HTTP
responses. Wait for handset receipt, and inspect `/api/v1/subscribers` to verify
the authoritative binding. `connections.number_source` identifies whether a
displayed number came from that registry or the volatile TMSI fallback.
短信首次注册与 API 改号的长度策略不同；无效号码或号码占用由短信回执提示。可在
`/api/v1/subscribers` 查权威绑定，连接列表的 `number_source` 表明号码来源。

For the single-container deployment, Asterisk must accept OpenBTS contacts at
`127.0.0.1`. The image ships a deny-by-default contact ACL with only this IPv4
loopback address allowed; external contacts and guest calling remain restricted.
An Asterisk `603`/contact-ACL failure after `101` may occur **after** number rows
were inserted; do not delete or recreate them just because confirmation was
missing. 同容器部署仅放行 OpenBTS 的本机联系人；101 后的联系人 ACL 失败可能发生
在号码入库之后，不能因为没收到确认就认定绑定未执行或删除重建签约。

Welcome SMS content is limited to a short ASCII instruction in this native
build. The configured message is followed by the native IMSI suffix; avoid
duplicating “Your IMSI is IMSI”. Historical welcome messages do not change when
the configuration changes, and `WELCOME_SENT` is not a delivery receipt.
本原生版本欢迎文字使用短 ASCII 操作提示，原生会追加 IMSI 标记，避免重复措辞；
修改配置不会改写手机历史短信，也不会把调度标记变成投递确认。

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

## Caller-number display / 来电号码显示

Both the realtime `phones` entry and `from-openBTS` initialize caller identity
from the configured peer resolved by `chan_sip`, then look up its bound number
through ODBC. Caller-controlled `P-IMSI`/display-name headers are not registry
lookup keys. Only a valid 2–15 digit mapping replaces caller ID and CDR identity;
unknown peers or invalid/empty mappings preserve the original values. The
outbound handset path no longer overwrites caller ID with an empty CDR field.
实时签约进入的 `phones` 与 `from-openBTS` 均通过 SIP 驱动解析的配置 peer 查
权威号码，不使用主叫自报的 IMSI 头或显示名；仅合法的 2–15 位数字映射更新来电
显示及 CDR。未知 peer、空值或异常映射保留原值，出局路径不再用空 CDR 覆盖号码。

`scripts/tests/test_callerid_image.sh` exercises the resolver with isolated
Asterisk Local channels and synthetic registry data, without RF or the live
database. This does not replace handset-to-handset incoming-number acceptance.
隔离 Asterisk Local 通道测试验证解析器及合成签约，不使用射频或生产库；仍须
两台手机互拨确认来电号码，不能只凭拨通或 Local 通道测试判定空口显示通过。

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

Postman binding/unbinding requests execute directly when Send is pressed; no
additional client enable switch is required. Use GET requests for read-only
inspection, and send each write individually rather than running the entire
collection. Postman 绑定/解绑点击 Send 即执行，无额外启用开关；只读检查使用
GET，写操作逐条发送，不要全量运行集合。
