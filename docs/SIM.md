# SIM / Subscribers SIM卡与签约 (explicit 显式语义)

No SIM-writer API in this system (unlike LTE `/writesim`). SIMs are written
externally; the Go side only manages the OpenBTS subscriber registry.
本系统无写卡接口，卡在外部写好；Go侧只管OpenBTS签约库。

- Volatile 易失：`/var/run/TMSITable.db` (`tmsi_table`: IMSI→ASSOCIATED_URI).
  Cleared on start/stop via `tmsis clear` + seed copy (see `entrypoint.sh`).
- Persistent 持久：`/var/lib/asterisk/sqlite3dir/sqlite3.db`
  (`sip_buddies.callerid`, `dialdata_table.exten` ↔ `IMSI<imsi>`).
  Seed: `SubscriberRegistry.db` → same file (`OpenBTS/smqueue.example.sql:62`).
- Config-only 仅配置：`/etc/OpenBTS/OpenBTS.db` (`CONFIG` table, radio params).

Use 使用：UE attaches → `POST /api/v1/subscribers {"imsi","number"}` (explicit
upsert, validated 15-digit IMSI / 2–15-digit number) → Asterisk voice + smqueue
SMS. Legacy `POST /setphonenumber` frozen with message_id 3/4.
