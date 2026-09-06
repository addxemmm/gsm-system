# QUICKSTART 快速开始 — pre-provisioned SIM + B210 (commands verified on vm-sdr 按实测为准)

> Status 状态：templates below verified locally for API shape (Go tests);
> RF steps marked `[ON-SERVER]` must be confirmed on `vm-sdr` before release.
> 以下模板本地已验API形状；标 `[ON-SERVER]` 的射频步骤须在服务器实测后更新本文。
> New integrations prefer `/api/v1`; legacy root shown in parentheses.

Prereq 前置 (`[ON-SERVER]`): Ubuntu 22.04, `docker compose`, B210 on USB3,
port `8082` free, SIMs pre-written externally (no writer API).

## 1. Health 健康检查 `[ON-SERVER]`

```bash
curl -s http://127.0.0.1:8082/api/v1/health; echo
# want {"code":0,"data":{"ok":true,"sdr":{"uhd_b210":true},...}}
```

## 2. Start 启动 `[ON-SERVER]`

```bash
# Explicit (first boot) 显式首次启动：
curl -X POST http://127.0.0.1:8082/api/v1/cell -H 'Content-Type: application/json' \
  -d '{"arfcns":"1","c0":"540","band":"1800","mcc":"001","mnc":"01","lac":"4420","ci":"41240","short_name":"test","network":"eth0"}'
# Reuse after recreate 重建后复用：
curl -X POST http://127.0.0.1:8082/api/v1/cell -d '{}'
# Legacy 旧版：curl -X POST http://127.0.0.1:8082/start -d '{}'
```

`network` = uplink iface (`ip route get 8.8.8.8` → `dev`).

## 3. Attach 入网 `[ON-SERVER]`

Insert SIM, manual network search, select MCC/MNC cell, enable data. Then:

```bash
curl -s http://127.0.0.1:8082/api/v1/ue | python3 -m json.tool
# Legacy: curl -X POST http://127.0.0.1:8082/ueinfo -d '{}'
```

## 4. Number + SMS + voice 号码/短信/互拨 `[ON-SERVER]`

```bash
curl -X POST http://127.0.0.1:8082/api/v1/subscribers -d '{"imsi":"001010123456780","number":"10000001"}'
curl -X POST http://127.0.0.1:8082/api/v1/sms -d '{"imsi":"001010123456780","sender":"101","text":"hello"}'
curl -s http://127.0.0.1:8082/api/v1/sms | python3 -m json.tool
# Voice 语音：two UEs dial each other's numbers via Asterisk (manual test).
```

## 5. Stop 停止

```bash
curl -X DELETE http://127.0.0.1:8082/api/v1/cell
```

## Failures 速查

| Symptom 现象 | Check 检查 |
|---|---|
| `503 no SDR` | `docker exec gsmsystem-v2 uhd_find_devices` (B210 present?) |
| `404 no UE` | band/SIM MCC-MNC match? manual search 3–5 min; prefer Android/CPE |
| `no SMS` | `docker exec gsmsystem-v2 tail /data/log/smqueue.log`; Chinese unsupported |
| `422 validation` | read `data.errors` per-field |
