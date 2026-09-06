# DEPLOY 部署 (SDR host, `vm-sdr`)

> Dev machine only edits + syncs; builds run on server. 本地只改+同步，构建在服务器。

## Prereq 前置 `[ON-SERVER]`

```bash
lsb_release -a; sudo docker --version; sudo docker compose version
lsusb | grep -i B210; sudo ss -tlnp | grep 8082 || echo "8082 free"
sudo docker ps --filter name=gsmsystem   # old 1.3 container must stay for rollback
sudo docker images | grep gsmsystem-dep  # 1.3 (6.25GB) kept
```

## Sync + build 同步与构建 (from Windows)

```powershell
.\scripts\deploy_from_windows.ps1 -HostAlias vm-sdr
# then on server 然后在服务器：
cd ~/gsm-system && sudo docker compose -f deploy/docker/docker-compose.yml build
sudo docker compose -f deploy/docker/docker-compose.yml up -d
curl -s http://127.0.0.1:8082/api/v1/health; echo
```

## Verify 验证 `[ON-SERVER]`

```bash
curl -s -X DELETE http://127.0.0.1:8082/api/v1/cell; echo
curl -s http://127.0.0.1:8082/api/v1/health; echo
# RF full chain 全链路：attach → ue → sms → voice (see QUICKSTART), then stop.
```

## Rollback 回滚

Old container `gsmsystem` + image `gsmsystem-dep:1.3` never removed until v2
full-chain passes. `up -d` with previous tag reverts.
