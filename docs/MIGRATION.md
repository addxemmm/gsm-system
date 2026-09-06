# Architecture: v1.x (Python) → v2.x (Go + OpenBTS) 架构说明

v1.x lives read-only in `gsmsystem/` + `gsmsystem_v1.3/` (HTTP + `run.sh`/`stop.sh`).
v1.x 只读存档于 `gsmsystem/` + `gsmsystem_v1.3/`。Mapping对照：

| v1.x | v2.x (this repo 本仓) | Notes 说明 |
|---|---|---|
| Python Flask `:8082` (`run.py` 671 lines) | `cmd/server` + `internal/api` (stdlib) | 10 routes + `message_id` frozen 冻结保留 |
| `os.popen` shell concat 拼接 | `exec.Command` argv + validation 校验 | injection-safe 防注入 (iface/IMSI/config) |
| `ps -aux \| grep OpenBTS` | `internal/sysop` pgrep + `Z` exclusion + `Wait()` reap | no self-match/zombie 不再误杀/僵尸误判 |
| `systemctl start` in container | direct binaries (no systemd) | container-safe 容器可用 |
| fixed `run_c.py` id crash | `gsm.Presets` bounds-checked | `run_c` int-concat bug fixed |
| index-based SMS/UE parse 下标解析 | `internal/parser` regex parse 正则解析 | tolerates log drift 容忍日志漂移 |
| `smqueue.db` ambiguous | start-clean + documented 启动重置文档化 | history from log, not DB |
| implicit subscriber writes 隐式写卡库 | explicit `POST /api/v1/subscribers` 显式API | `TMSITable` volatile vs `sqlite3.db` persistent documented |
| `192.168.99.0/24` MASQUERADE ad-hoc | same rule, validated iface | behavior kept, iface checked |
| Chinese SMS silently missing | v1 explicit 422 non-GSM7 | documented, not silent 不再静默 |

Legacy assets 旧资产：`gsmsystem/` (v1 Flask + OpenBTS dumps + asterisk confs +
`smqueue_5.0_amd64.deb`), `gsmsystem_v1.3/` (manual-start scripts + DB-reset
`stop.sh`), `TempFile/` (DB copies), `docs/legacy/README.v1.md` (520-line
Chinese manual, moved from root).

## folk/uhd4: Xenial+UHD3.9 → 22.04+UHD4.1 (clone-board line, current live)

The GSM-VM board is an Artix-7 BlackSDR mini clone (s/n 2548123): the stock
Spartan-6 FPGA image loads 100% but the FX3 reports config failure
(`fx3 is in state 5`, proven with old image 1.0 too — same driver, same file,
same failure). Only the 4.x-era clone image configures it, so the radio
stack moved to UHD 4.1.0.0 on Ubuntu 22.04:

| Xenial line 主线 (`Dockerfile`) | uhd4 line (`Dockerfile.uhd4`, live) | Notes 说明 |
|---|---|---|
| `ubuntu:16.04`, gcc-5 | `ubuntu:22.04`, gcc-11, `-std=gnu++14 -w -fpermissive` | era-correct standard; `-Werror` off for 2019 code |
| UHD `release_003_009_000` | UHD `v4.1.0.0` (host-prefetched, hermetic) | xenial TLS can't handshake github |
| stock `usrp_b210_fpga.bin` | clone `usrp_b210_fpga.blacksdr.bin` + `usrp_b200_fw.hex`/`.bl.img` (`firmware/uhd/`) | FX3 firmware required by UHD 4.x |
| `uhd::msg` as upstream | `compat/uhd/utils/msg.hpp` shim | removed in UHD 4.0; one call site |
| glibc-era `gettid` fallback | `compat/patches/0001` (+ loop over all 4 vendored copies) | glibc ≥ 2.30 provides it |
| ortp 1.x API | `compat/patches/0002` (bctbx log hook) + `-DORTP_NEW_API=1` | upstream already branched for it |
| ostringstream streaming | 6× `.str()` (L3StateMachine/MSInfo/Sgsn) | illegal since C++11 |
| `liba53` deb / coredumper deb | plain `make install` / manual deb steps | 2007-era debian/ rejected by debhelper ≥ 7; glibc dropped `sys/sysctl.h` |
| image 6.25GB (legacy) / 484MB | image ~410MB, zero Python | multi-stage, sources stay in builder |

`gsmsystem-dep:2.0` (xenial line) stays on host as rollback; it needs a
genuine Spartan-6 B210 (verified: cannot drive the clone board).
