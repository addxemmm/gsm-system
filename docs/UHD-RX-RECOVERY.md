# UHD receive timeout recovery / UHD 接收超时恢复

## Observed failure / 已观察故障

The pinned OpenBTS UHD adapter treated one empty `recv(..., 0.1, true)` timeout
as unrecoverable and immediately exited the transceiver. The observed next
failure was OpenBTS's TRX clock watchdog about nine seconds later. The remaining
SIP/SMS/voice helper processes made the cell status `degraded`, not `stopped`.
This chain was observed more than once; it is not evidence that Docker networking
caused the radio failure.

固定版本驱动将一次约 100 毫秒的空接收超时直接视为致命错误，退出 transceiver；
约九秒后 OpenBTS 因 TRX 时钟超时退出。辅助进程仍在，因此接口显示 degraded。
日志中重复出现这条故障链，但这不代表故障由 Docker bridge 引起。

UHD defines `ERROR_CODE_TIMEOUT` as no packet received before the implementation
timeout, not proof that the device is permanently disconnected. See the
[official RX metadata reference](https://files.ettus.com/manual/structuhd_1_1rx__metadata__t.html)
and [receive API](https://files.ettus.com/manual/classuhd_1_1rx__streamer.html).
UHD 的超时语义是等待期间未收到包，并不等于已经确认设备永久断开。

## Bounded recovery / 有限恢复

- Retry empty timeout results in the same receive loop, with a limit of ten
  timeouts and a 1,000 ms steady-clock budget. The original 100 ms receive-call
  timeout remains. Scheduling or driver blocking can exceed nominal wall time;
  this is not a real-time deadline guarantee.
  / 在同一接收循环内有限重试，最多十次超时并检查单调时钟 1,000 毫秒预算；保留原
  100 毫秒调用等待。线程调度或驱动阻塞可能超出名义时长，不承诺硬实时截止。
- Require the first recovered packet to continue exactly from the previous
  accepted packet's end tick. Otherwise the legacy sample ring could expose
  stale/uninitialized I/Q across a timestamp gap. A gap, overlap or missing
  timestamp remains fatal; no synthetic samples are inserted.
  / 恢复首包必须与前包结束位置连续；否则旧环形缓冲可能把空洞里的旧数据当成有效
  I/Q。时间断档、重叠或缺失时间戳仍报错，不伪造采样数据。
- Log the initial wait, successful recovery or exhausted budget. Do not restart
  RF, reset the device clock, or automatically restart a failed container.
  / 记录首次等待、恢复或耗尽预算，不盲目重启射频、重置设备时钟或重启容器。

This fixes premature process exit for a transient timeout followed by contiguous
samples. It does not fix a persistent USB/FPGA failure, actual sample loss, a
transmit timeout, or unrelated RF faults. Sustained failure must remain visible.
本修复处理“短暂超时后收到连续样本”造成的过早退出，不把持续 USB/FPGA 故障、真实
丢样、发送超时或其他射频故障宣称为已解决；持续异常仍须明确报告。

## Validation / 验证

The native image build applies `0005-uhd-rx-timeout-retry.patch` and compiles
the actual patched receive methods with scripted input fixtures through
`compat/tests/test-uhd-rx-timeout.sh`. Test cases inject transient/permanent
timeouts and invalid/continuous timestamps without accessing SDR hardware.
原生镜像构建应用 0005 补丁，并提取实际生产接收方法编译故障注入测试，不接触射频硬件。

After deployment the operator starts the cell through its existing preset or
custom start API. Inspect native logs for `Receive recovered after`,
`Receive timeout persisted` or `Discontinuous receive after timeout`. Save the
corresponding USB/kernel evidence if failure persists. A healthy management
container alone is not proof of sustained RF stability or handset service.
部署后由操作者通过预设或自定义接口启动小区，观察上述恢复或失败日志；持续异常时
结合 USB 与内核记录继续定位。管理容器健康不等于已验证长时间射频稳定或手机业务。
