# GPRS identity exchange and NAT recovery / GPRS 身份交换与 NAT 恢复

## Observed failure / 已观察故障

The September 9 inspection found working circuit-switched calls/SMS but no
observed handset packet IP. GPRS was enabled and container forwarding was on,
yet the GSM MASQUERADE rule was absent after restart. Native logs repeatedly
reported `Attempt to send TBF assignment on CCCH to MS without an IMSI?`, followed
by failed downlink TBFs. The SGSN snapshot contained a deregistered context with
no PDP IP. These are two different failure points; repairing NAT alone does not
complete GPRS attachment. Container HTTP and an independent HTTPS probe worked,
which establishes container egress, not handset egress.
9 月 9 日观察到通话/短信正常、手机数据 IP 为空；GPRS 与转发开启但重启后缺少 NAT。
原生日志反复报分配下行 TBF 时缺少 IMSI，SGSN 上下文未注册且无 PDP IP。身份交换与
出口 NAT 是两个独立问题，补规则不等于已完成数据附着；容器能访问网页也不等于手机能上网。

## Pre-IMSI assignment / 尚未取得 IMSI 时的信道分配

A handset can begin packet registration with a temporary identity. The SGSN
then needs to send an Identity Request before it knows the IMSI. In the pinned
public OpenBTS source, `sendAssignmentCcch` returned immediately on an empty
IMSI, blocking the very downlink needed to obtain it.
手机可使用临时身份发起分组注册，SGSN 需要先下发身份请求才能取得 IMSI；固定公开版
却在 IMSI 为空时提前返回，阻塞了获取身份所必需的下行过程。

Patch `0007-gprs-pre-imsi-ccch-assignment.patch` removes that early return only
for the existing GPRS Immediate Assignment path. The public-release paging queue
sends its prebuilt assignment addressed by TLLI, not an IMSI-based GSM page.
It does not invent an IMSI, copy circuit-switched registration identities, grant
subscriber admission, change authentication or merge foreign/local TLLIs by guess.
补丁仅修正既有 GPRS Immediate Assignment 路径的提前返回；公开版队列发送已按 TLLI
寻址的预构造分配，不是依赖 IMSI 的普通 GSM 寻呼。不伪造身份、不借用语音注册库、
不修改接入鉴权，也不猜测合并不同临时身份。

This reasoning is specific to the pinned public pager. If upstream adds IMSI
paging groups, revalidate this patch and its queue/constructor assumptions before
changing the source revision. Normal circuit-switched paging remains unchanged.
本修复依赖固定公开版寻呼实现；上游引入 IMSI 分组后须重新审计队列和构造函数假设，
不得无条件沿用。普通语音寻呼不变。

## Network gate / 网络前置检查

All explicit cell-start forms now call the same idempotent GSM NAT helper after
input/interface/hardware checks and before native service launch. The rule is
limited to source `192.168.99.0/24`, the selected container uplink and MASQUERADE.
It is added only when absent; command failure returns HTTP 503 without launching
native services. The API `PUT /network` shares that helper and retains its stopped
precondition. No sysctl or global firewall policy is changed.
三种显式启动共用相同幂等 NAT 实现，在输入/网卡/硬件检查后、原生进程启动前确保
限定源网段与出口网卡的规则存在；命令失败返回 503，不启动服务。显式网络接口复用同一
实现且保留停止态要求，不改 sysctl 或全局防火墙策略。

`persisted:false` remains accurate: rules live in the network namespace, not a
saved firewall service. A later explicit start restores a missing rule; there is
no automatic container or RF restart. `restart: "no"` remains the Compose policy.
规则仍是运行态，因此 persisted:false 不变；下次显式启动补齐缺失规则，不增加容器或
射频自启，Compose 继续使用 restart:"no"。

## Verification boundaries / 验证边界

- Native regression must cover unknown/known IMSI, preserve TLLI targeting,
  exercise relevant production queue/constructor methods and demonstrate the
  original-source failure. / 原生回归覆盖未知/已知身份、临时身份寻址与相关生产队列/构造
  路径，并以原代码复现阻塞。
- Go regression checks NAT creation/idempotence/failure and startup ordering.
  The isolated SMS image suite uses an inert firewall fixture, not real RF or
  production firewall rules. / Go 验证规则创建、幂等、失败与启动顺序；隔离镜像使用惰性
  防火墙样本，不操作生产规则或射频。
- Live acceptance needs handset reattachment, registered SGSN/PDP state, a
  packet IP, bidirectional TUN/NAT traffic, DNS resolution and a small web page.
  Phone call/SMS success alone is insufficient. / 真机仍须逐层验证附着、PDP 地址、双向
  流量、DNS 与小网页，不用通话/短信正常替代上网验收。

See [SIM setup](SIM.md), [operations](OPERATIONS.md), and the dated
[release record](RELEASE-2.1.md) for actual validation results.
手机设置、操作流程及实际验收结果分别查看以上文档和有时间标记的发布记录。
