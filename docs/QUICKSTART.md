# QUICKSTART 快速开始 — pre-provisioned SIM + B210 (verified on vm-sdr 按实测为准)

> New integrations prefer `/api/v1`; legacy root shown in parentheses.
> Live stack: `gsmsystem-uhd4` container, image `gsmsystem-uhd4:test`
> (22.04 + UHD 4.1 + OpenBTS 5.0 ported, Artix-7 clone FPGA bundled).
> Steps 1–2 verified live 2026-09-06; steps 3–4 need real handsets.

Prereq 前置：Ubuntu 22.04 host, `docker compose`, B210 on USB3
(`lsusb` → `2500:0020`), port `8082` free, SIMs pre-written externally
(no writer API). No `sudo` needed (user is in the `docker` group).

## 1. Health 健康检查 ✓ verified

```bash
curl -s http://127.0.0.1:8082/api/v1/health; echo
# {"code":0,"message":"ok","data":{"ok":true,"running":false,
#  "sdr":{"uhd_b210":true,...}},...}
```

`uhd_b210` must be `true` before `/start`, else `503`.
If the board vanishes from USB (unbind crash), re-plug it physically —
see `SDR.md` §USB recovery. Do NOT unbind while FPGA is loaded.

## 2. Start 启动 ✓ verified

```bash
# Explicit (first boot) 显式首次启动 (vm-sdr uplink is ens33):
curl -X POST http://127.0.0.1:8082/api/v1/cell -H 'Content-Type: application/json' \
  -d '{"arfcns":"1","c0":"540","band":"1800","mcc":"001","mnc":"01","lac":"4420","ci":"41240","short_name":"test","network":"ens33"}'
# verified response: {"code":0,"message":"cell started",...}
# Reuse after recreate 重建后复用：
curl -X POST http://127.0.0.1:8082/api/v1/cell -d '{}'
# Legacy 旧版：curl -X POST http://127.0.0.1:8082/start -d '{}'
```

`network` = uplink iface (`ip route get 8.8.8.8` → `dev`).
Verify all five daemons: `GET /api/v1/cell` →
`{running:true,openbts:true,transceiver:true,sipauthserve:true,smqueue:true,asterisk:true}`.

## 3. Attach 入网 (needs handsets 待真机)

Insert SIM, manual network search, select MCC/MNC cell, enable data. Then:

```bash
curl -s http://127.0.0.1:8082/api/v1/ue | python3 -m json.tool
# want {"code":0,"data":{"ues":[{"imsi":"...","imei":"...","number":"...","ip":"..."}],"count":1}}
# Legacy: curl -X POST http://127.0.0.1:8082/ueinfo -d '{}'
```

## 4. Number + SMS + voice 号码/短信/互拨 (needs handsets 待真机)

```bash
curl -X POST http://127.0.0.1:8082/api/v1/subscribers -d '{"imsi":"001010123456780","number":"10000001"}'
curl -X POST http://127.0.0.1:8082/api/v1/sms -d '{"imsi":"001010123456780","sender":"101","text":"hello"}'
curl -s http://127.0.0.1:8082/api/v1/sms | python3 -m json.tool
# Voice 语音：two UEs dial each other's numbers via Asterisk (manual test).
```

## 5. Stop 停止

```bash
curl -X DELETE http://127.0.0.1:8082/api/v1/cell
# {"code":0,"message":"cell stopped","data":{"stopped":true}}
```

## Failures 速查

| Symptom 现象 | Check 检查 |
|---|---|
| `503 no SDR` | `docker exec gsmsystem-uhd4 uhd_find_devices` (B210 present?) |
| `fx3 is in state 5` | wrong FPGA for the board: Artix-7 clone needs the clone image (this stack bundles it); see `SDR.md` |
| USB device gone | physical re-plug (no software reset while FPGA loaded); see `SDR.md` §USB recovery |
| `404 no UE` | band/SIM MCC-MNC match? manual search 3–5 min; prefer Android/CPE |
| `no SMS` | `docker exec gsmsystem-uhd4 tail /data/log/smqueue.log`; Chinese unsupported |
| `422 validation` | read `data.errors` per-field |
