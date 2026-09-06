# DEPLOY 部署 (SDR host, `vm-sdr`)

> Dev machine only edits + syncs; builds run on server. 本地只改+同步，构建在服务器。
> No `sudo` anywhere below (user is in `docker`; passworded sudo fails over ssh).
> Live line: `folk/uhd4` → image `gsmsystem-uhd4:test` → container `gsmsystem-uhd4`
> (22.04 + UHD 4.1 + ported OpenBTS, Artix-7 clone FPGA bundled).

## Prereq 前置 (on server)

```bash
lsb_release -a; docker --version; docker compose version
lsusb | grep 2500              # B210 present?
ss -tlnp | grep 8082 || echo "8082 free"
docker ps -a --format '{{.Names}} | {{.Image}} | {{.Status}}'
docker image ls --format '{{.Repository}}:{{.Tag}} {{.Size}}'
```

Kept on host: `gsmsystem-uhd4:test` (live), `gsmsystem-dep:2.0` (485MB
xenial fallback), `ltesystem-dep:2.0` (LTE), base images. Legacy
`gsmsystem-dep:1.x` (6.25GB) removed 2026-09-06 (seeds/configs extracted to git).

## First time: vendor sources 首次预取 (on server, host git works)

```bash
cd ~/gsm-system && ./scripts/prefetch_vendor.sh   # fills third_party/ (~450MB, untracked)
```

## Sync + build 同步与构建 (from Windows dev machine)

```powershell
.\scripts\deploy_from_windows.ps1 -HostAlias vm-sdr   # git-archive sync (OneDrive breaks tar)
# then on server 然后在服务器：
cd ~/gsm-system && docker compose -f deploy/docker/docker-compose.uhd4.yml build
docker compose -f deploy/docker/docker-compose.uhd4.yml up -d
curl -s http://127.0.0.1:8082/api/v1/health; echo
```

`docker.io` is flaky from this host; on manifest errors just retry the build
(cached layers resume).

## Verify 验证 (on server)

```bash
curl -s http://127.0.0.1:8082/api/v1/health; echo        # uhd_b210 must be true
docker exec gsmsystem-uhd4 timeout 90 uhd_usrp_probe 2>&1 | grep -E 'loopback|Error|compat'
# RF full chain 全链路：cell start → ue → sms → voice (see QUICKSTART), then stop.
```

## Rollback 回滚

`gsmsystem-dep:2.0` image stays on host until the uhd4 line passes the full
RF chain. Revert: `docker compose -f deploy/docker/docker-compose.yml up -d`
(xenial line, needs a genuine Spartan-6 B210). Config/profile live in the
`docker_gsm-data` volume — kept across rebuilds; history in git.
