# SDR and RF / SDR 与射频

## Supported line / 支持范围

- Hardware: USRP B210-compatible USB3 device; Compose passes `/dev/bus/usb`
  with required privileges and uses an isolated Docker bridge.
- Runtime: Ubuntu 22.04, UHD 4.1, native OpenBTS 5.0 and Asterisk.
- The project-owned management plane is Go. OpenBTS/UHD remain C/C++, and
  upstream UHD uses Python/Mako in the Docker builder stage only.
- Release 2.1 has one production `deploy/docker/Dockerfile` and one
  `deploy/docker/docker-compose.yml`; the former `.uhd4` definitions are retired.

硬件与射频栈没有“全部改写为 Go”；Go 范围仅是本项目管理面。UHD 构建阶段仍使用
Python/Mako，运行时继续使用原生 OpenBTS/Asterisk。

## Radio constraints / 射频限制

| Field | Release 2.1 rule |
|---|---|
| `arfcns` | exactly `1` / 固定为 1 |
| GSM 900 `c0` | `0..124` or `975..1023` |
| DCS 1800 `c0` | `512..885` |
| `lac` | `1..65279` software-compatible range / 软件兼容范围 |
| `ci` | `0..65535` |

Before any `POST /api/v1/cell`, verify local spectrum permission, the correct
antenna and band, a physical RF kill path, and safe separation. Never leave a
transmitting cell unattended. 发射前确认频谱许可、天线/频段、硬件断射频手段与安全距离。

## Health and ownership / 健康与设备占用

`GET /api/v1/health` is an HTTP/control-plane health report.
`GET /api/v1/cell` reports managed process state:

- `state`: `stopped|transitioning|degraded|running`;
- `ready`: all five managed processes are alive;
- `sms_ready`: OpenBTS and smqueue are alive;
- `voice_ready`: OpenBTS and Asterisk are alive.

These flags do not claim RF lock, successful transmit, handset attach, SMS
delivery, or voice quality. Only run an exclusive `uhd_usrp_probe` when the cell
and transceiver are stopped; a busy probe while the transceiver owns the B210 is
not evidence that the device disappeared. 就绪状态只描述进程，不是射频验收结论。

## Network path / 数据网络

```http
GET /api/v1/network?iface=eth0
PUT /api/v1/network
Content-Type: application/json

{"iface":"eth0"}
```

`PUT` validates the interface and applies the managed MASQUERADE rule
idempotently while the cell is stopped. `eth0` is the container's bridge
uplink, not a host NIC. OpenBTS assigns handset data from `192.168.99.0/24`;
the rule translates that source to the GSM container address, after which
Docker's bridge performs normal outbound translation on the host. Container
IPv4 forwarding is enabled explicitly by Compose.

`persisted:false` means the managed rule is runtime state in the **container
network namespace** and must be reapplied after container recreation. The Go
process neither edits the host's global Docker firewall nor accepts an
arbitrary shell fragment: the validated interface is one argv value. 当前固定
出口为容器内 `eth0`，而非宿主机网卡；MASQUERADE 规则属于容器网络
命名空间，容器重建后需重新应用，不会改写宿主机 Docker 全局防火墙。

Only the Go management API is published as host TCP 8082. The native
loopback/intra-container paths remain private: Asterisk SIP 5060, OpenBTS SIP
5062 and RTP 16484–16581, smqueue 5063, sipauthserve 5064, OpenBTS CLI 49300,
TRX 5700, and Asterisk RTP 20000–22000. External trunks, handover peers, or
remote RTP require a separate reviewed deployment profile; they are not part
of the default single-container contract. 默认部署不向宿主机发布任何
SIP、RTP、CLI 或 TRX 端口。

## Board-specific historical note / 板卡历史记录

The lab previously verified an Artix-7 B210-compatible board with a matching
clone FPGA image. A stock Spartan-6 image can load but be rejected by FX3. Keep
the board-matched image pair and never infer compatibility from upload progress
alone. Specific serials, host interfaces, and live-stack names are deliberately
omitted from the 2.1 contract. 实验室曾验证 Artix-7 兼容板需匹配的 FPGA/FX3 镜像；
序列号与实际网卡不属于公开契约。

If a USB device wedges or vanishes, stop the cell and recover at the physical or
hypervisor USB layer. Do not unbind a live device while its FPGA/transceiver is
active. 设备被收发器占用时禁止进行破坏性 USB 解绑操作。

## Evidence to retain / 验收证据

For an SDR-host release acceptance, retain image ID/tag, Compose status,
`GET /health`, cell lifecycle responses, process/native logs, attach evidence,
SMS submission plus independent receipt evidence, completed-call CDR, and final
stop/persistence checks. Do not mark 2.1 verified from local Go tests alone.
