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
