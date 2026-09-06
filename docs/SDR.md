# SDR 射频 (B210 only)

- Hardware 硬件：USRP B210 (USB3), `--privileged --net=host -v /dev/bus/usb`.
- Detect 检测：`uhd_find_devices` contains `B210` (`internal/sdr`: `Detect()`).
  `/api/v1/health.sdr.uhd_b210` must be true before `/start`; else `503/3`.
- Data path 数据面：GSM voice/SMS via OpenBTS+Asterisk; UE data via
  `192.168.99.0/24` MASQUERADE on uplink `network` iface (`POST /api/v1/network`).
- Logs 日志：`/data/log/openbts.log` (`system ready` gate), `/data/log/smqueue.log`.
- Discipline 纪律：confirm antenna/band, keep distance, never leave transmitting
  unattended; stop after tests.

## Board variant 板型 (vm-sdr, verified 2026-09-06 实测)

| Item | Value |
|---|---|
| Board 板卡 | BlackSDR mini clone (Artix-7; NOT Spartan-6) |
| Serial 序列号 | `2548123` (`name=b210mini, product=B210`) |
| USB | USB 3.0 SuperSpeed, VM bus 4 (`4-1`), usbfs 256MB |
| Uplink 上行网卡 | `ens33` (`ip route get 8.8.8.8` → `dev`) |
| Live stack 现行栈 | `gsmsystem-uhd4`: UHD 4.1.0.0 + clone FPGA `firmware/uhd/usrp_b210_fpga.blacksdr.bin` + `usrp_b200_fw.hex` |

The stock Spartan-6 image loads 100% on this board but the FX3 rejects it
(`Error: RuntimeError: fx3 is in state 5`); the old 1.x stack fails
identically (same driver, same file — proven, not a driver-version issue).
Do NOT use the stock image here; the uhd4 image bundles the right pair.

## USB recovery USB 恢复

- Never unbind the USB device while the FPGA is loaded (a wedged FX3
  re-enumerates as an empty shell: `bcdDevice 0.00`, invisible to UHD).
  Never 在 FPGA 加载状态下解绑 USB。
- If the board vanishes from `lsusb`: re-plug it physically (or re-attach
  it to the VM in the hypervisor). Software xHCI rebind does NOT revive a
  wedged FX3 — learned the hard way 2026-09-06.
- Recovery check 恢复确认：`lsusb | grep 2500` → `uhd_find_devices` →
  `uhd_usrp_probe` (register loopback passed ×2).
