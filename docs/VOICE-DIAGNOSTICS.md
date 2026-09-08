# Local voice diagnostics / 本机语音诊断

| Number / 号码 | Function / 功能 | Limit / 上限 |
|---|---|---|
| 2600 | Echo: hear your own voice; press # to exit / 回声，按 # 结束 | 60 seconds / 秒 |
| 2602 | Digital milliwatt test tone / 数字毫瓦测试音 | 30 seconds / 秒 |

These are built-in Asterisk services, not subscribers. No number binding,
external SIP trunk, prompt package or second handset is required. The call
must still reach Asterisk through a working GSM signaling and voice channel.
以上是 Asterisk 内置服务，不是手机号码；无需绑定号码、外部中继、语音文件包或第二部
手机，但仍依赖可用的 GSM 信令与语音信道。

Both numbers have exact routes in the `phones` and `default` contexts, so
the generic subscriber lookup cannot route them toward an absent PSTN trunk.
The old broad demo file remains excluded: it includes shell commands, host
information and recording helpers unrelated to these two diagnostics.
两个入口都使用精确分机规则，避免测试号码落入注册库通配查询或不存在的 PSTN 中继。
旧的大型演示文件仍不加载，不恢复其中的 shell、宿主机信息或录音功能。

The number-binding API reserves these numbers and returns HTTP 422. Existing
bindings must be checked for conflicts before deploying restored service routes;
do not silently reassign a subscriber's number.
号码绑定接口将其保留并返回 422；部署前检查现有绑定是否冲突，不静默改号。

`test_callerid_image.sh` originates synthetic Local calls through `phones`,
`default`, and `from-openBTS`, verifies answered Echo/Milliwatt execution, and
cleans up the test container. This proves routing/modules, not RF audio quality.
隔离镜像测试通过三个入口发起 Local 通道，检查已接通的 Echo/Milliwatt 应用并清理资源；
它验证路由与模块，不替代手机空口音频测试。

Live acceptance: start the cell, attach a test SIM, dial 2600 and then 2602.
Confirm echo/tone before testing a second handset. If both built-in calls fail,
inspect OpenBTS channel allocation, radio continuity and SIP response codes
before changing peer-number routing. The radio timeout patch is documented in
[UHD-RX-RECOVERY.md](UHD-RX-RECOVERY.md).
真机验收：启动小区并接入测试卡，依次拨打 2600 和 2602，确认回声/测试音后再测互拨。
若这两个本机服务也失败，应优先检查无线连续性、信道分配和 SIP 响应，而非只改号码路由。

## Outstanding live acceptance / 尚待真机验收

The pre-update production log also showed TCH reassignment targeting a channel
still in RequestRelease/ReassignTarget, L2 ERROR, a late paging response without
an MM record, and peer-call SIP 502/cause 27. Those are different from the missing
test-number route (cause 3). The bounded RX timeout patch is not evidence that
every radio allocation or paging fault has been repaired.
升级前日志还出现目标语音信道尚未释放、L2 错误、寻呼响应晚于 MM 记录生命周期，
以及互拨 SIP 502/cause 27。这与测试号码缺路由的 cause 3 不同，不能把 RX 重试补丁
等同于全部无线分配或寻呼问题已经修复。

The pinned smqueue delivery state machine waits 15 seconds for confirmation,
then 60 seconds before a failed attempt is retried; an address-lookup error can
wait 300 seconds. These explain how a missed delivery can become a long delay,
but current NOTICE logs do not attribute a particular handset SMS to that path.
The read-but-unused `SIP.Timeout.MessageResend` setting does not override this
state table. This release does not silently tune these timers.
固定版本 smqueue 投递确认等待 15 秒，失败后再等 60 秒重试，部分地址查询失败会等待
300 秒；这解释了失败后延迟放大的可能机制，但现有 NOTICE 日志尚未关联到用户某条
短信。读取后未应用的 SIP.Timeout.MessageResend 配置也不会覆盖该状态表，本次不静默
调整这些定时器。

For acceptance, use distinct test texts and note send/receive times for each
direction and API-to-handset delivery. Correlate the transaction's paging,
TCH assignment, SIP result and ACK before choosing a timer or radio adjustment.
A fast local 101 service response does not establish another handset's MT
paging health. Keep existing bindings and GPRS settings during comparison.
验收时用不同测试正文并记录双向及 API 下发的收发时刻，关联事务的寻呼、信道分配、
SIP 结果与确认，再决定是否调整参数。101 本机服务回复快，不代表另一部手机的下行
寻呼正常；对照测试期间保持原有绑定和 GPRS 配置。
