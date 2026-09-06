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
`stop.sh`), `TempFile/` (DB copies), `README.md` (520-line Chinese manual, kept).
