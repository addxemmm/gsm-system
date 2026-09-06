# SDR 射频 (B210 only)

- Hardware 硬件：USRP B210 (USB3), `--privileged --net=host -v /dev/bus/usb`.
- Detect 检测：`uhd_find_devices | grep B210` (`internal/sdr`: `Detect()`).
  `/api/v1/health.sdr.uhd_b210` must be true before `/start`; else `503/3`.
- Data path 数据面：GSM voice/SMS via OpenBTS+Asterisk; UE data via
  `192.168.99.0/24` MASQUERADE on uplink `network` iface (`POST /api/v1/network`).
- Logs 日志：`/data/log/openbts.log` (`system ready` gate), `/data/log/smqueue.log`.
- Discipline 纪律：confirm antenna/band, keep distance, never leave transmitting
  unattended; stop after tests. Full topo snapshot pending `[ON-SERVER]` probe.
