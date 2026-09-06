# DEPLOY / 部署 (SDR host, `vm-sdr`)

> Dev machine only edits + syncs; builds run on server. 本地只改与同步，构建在服务器。
> No `sudo` below: the server user must belong to `docker`; passworded sudo fails over SSH.
> 以下命令不使用 `sudo`：服务器用户须属于 `docker` 组，SSH 下带密码的 sudo 会失败。
> Live line: `folk/uhd4` → image `gsmsystem-uhd4:test` → container `gsmsystem-uhd4`
> (22.04 + UHD 4.1 + ported OpenBTS, Artix-7 clone FPGA bundled).

## Prerequisites / 前置条件 (on server / 服务器)

```bash
lsb_release -a; docker --version; docker compose version
rsync --version | head -1      # required by the transactional Windows sync / Windows 同步需要
lsusb | grep 2500              # B210 present?
ss -tlnp | grep 8082 || echo "8082 free"
docker ps -a --format '{{.Names}} | {{.Image}} | {{.Status}}'
docker image ls --format '{{.Repository}}:{{.Tag}} {{.Size}}'
```

Kept on host: `gsmsystem-uhd4:test` (live), `gsmsystem-dep:2.0` (485MB
xenial fallback), `ltesystem-dep:2.0` (LTE), base images. Legacy
`gsmsystem-dep:1.x` (6.25GB) removed 2026-09-06 (seeds/configs extracted to git).

## First-time vendor sources / 首次预取依赖源码

```bash
cd ~/gsm-system && ./scripts/prefetch_vendor.sh   # fills third_party/ (~450MB, untracked)
```

## Safe sync and build / 安全同步与构建 (Windows dev machine / 开发机)

```powershell
# Sync committed HEAD only; does not build or touch containers.
# 只同步已提交 HEAD；不构建，也不启动或替换容器。
.\scripts\deploy_from_windows.ps1 -HostAlias vm-sdr

# Sync and build the live UHD4 image; still does not run compose up.
# 同步并构建当前 UHD4 镜像；仍不执行 compose up。
.\scripts\deploy_from_windows.ps1 -HostAlias vm-sdr -Build

# Archival Xenial build (genuine B210 only) / 归档 Xenial 构建（仅原装 B210）：
.\scripts\deploy_from_windows.ps1 -HostAlias vm-sdr -Build `
  -ComposeFile deploy/docker/docker-compose.yml
```

The default compose file is `deploy/docker/docker-compose.uhd4.yml`, matching the
live Artix-7/UHD4 line. `-ComposeFile` accepts a safe repository-relative path.
默认 Compose 文件对应当前 Artix-7/UHD4 实机；可用 `-ComposeFile` 明确选择其他配置。

Sync uses a unique local/remote archive and staging directory, checks every
`git`/`scp`/`ssh` exit code, and overlays committed `HEAD` with `rsync -a`.
It does not delete unknown server files, and retains the large server-only
`third_party/` build cache. If a committed path was removed, review and remove
that stale server path explicitly. 本流程检查所有原生命令退出码，用 `rsync -a` 覆盖
已提交的 `HEAD`，不删除未知服务器文件并保留 `third_party/` 缓存；仓库若删除了已提交
路径，须复核后在服务器明确清理对应陈旧路径。

`git archive` intentionally excludes modified and untracked files. The script
prints a warning listing them; commit intended changes before deployment.
`git archive` 不包含未提交与未跟踪文件；脚本会逐项警告，部署前请提交需要同步的改动。

`docker.io` is flaky from this host; on manifest errors retry the build (cached
layers resume). 若镜像清单下载失败，可重试构建并复用已缓存层。

## One-time persistent-state migration / 一次性持久化迁移

**Do not recreate a pre-persistence UHD4 container directly.** First make SQLite
online backups of all three OpenBTS databases and Asterisk, writing both the new
state files and a retained timestamped backup into the existing `docker_gsm-data`
volume. 旧版 UHD4 容器升级时严禁直接 recreate；必须先备份三份 OpenBTS 数据库和
Asterisk 数据库，再切换容器。

Before starting the backups, stop the cell through the normal management flow
and pause subscriber/configuration write requests. Each `.backup` is transactionally
consistent, but the four independent files are not a single cross-database instant.
备份前按正常管理流程停止小区，并暂停签约/配置写请求；单个 `.backup` 具备事务一致性，
但四个数据库的备份并非同一时刻的跨库快照。

Run while the old container is still running (replace `old` only if its name is
different). 在旧容器仍运行时执行（名称不同才修改 `old`）：

```bash
old=gsmsystem-uhd4
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker volume inspect docker_gsm-data >/dev/null
docker inspect "$old" --format '{{range .Mounts}}{{println .Name .Destination}}{{end}}'
docker exec "$old" sh -ceu '
stamp=$1
umask 077
backup=/data/backups/pre-persistence-$stamp
items="/etc/OpenBTS/OpenBTS.db:OpenBTS/OpenBTS.db
/etc/OpenBTS/sipauthserve.db:OpenBTS/sipauthserve.db
/etc/OpenBTS/smqueue.db:OpenBTS/smqueue.db
/var/lib/asterisk/sqlite3dir/sqlite3.db:asterisk/sqlite3.db"
for item in $items; do
  src=${item%%:*}; rel=${item#*:}; dst=/data/state/$rel
  [ -f "$src" ] || { echo "missing source database: $src" >&2; exit 1; }
  [ ! -e "$dst" ] || { echo "refusing to overwrite: $dst" >&2; exit 1; }
done
for item in $items; do
  src=${item%%:*}; rel=${item#*:}; dst=/data/state/$rel
  mkdir -p "$(dirname "$backup/$rel")" "$(dirname "$dst")"
  sqlite3 "$src" ".backup '\''$backup/$rel'\''"
  sqlite3 "$src" ".backup '\''$dst'\''"
  chmod 700 "$(dirname "$backup/$rel")" "$(dirname "$dst")"
  chmod 600 "$backup/$rel" "$dst"
  [ "$(sqlite3 "$backup/$rel" "PRAGMA quick_check;")" = ok ]
  [ "$(sqlite3 "$dst" "PRAGMA quick_check;")" = ok ]
done
echo "retained backup: $backup"
' sh "$stamp"
```

Only after all eight backup/verification operations succeed, stop and retain the
old UHD4 container, then launch the new Compose project. 全部备份和校验成功后，才停止
并保留旧 UHD4 容器，再启动新 Compose 项目：

```bash
docker stop --time 90 "$old"
docker rename "$old" "$old-pre-$stamp"   # stopped rollback container / 停止态回滚容器
cd ~/gsm-system
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live --skip-build
```

The UHD4 compose file pins `${GSM_DATA_VOLUME:-docker_gsm-data}`, so the new
project uses the migrated volume while Compose does not adopt/delete the renamed
old-project container. Do not delete the timestamped `/data/backups/` directory,
old image, or stopped old container until full RF acceptance. UHD4 Compose 使用固定卷名，
新项目能读取迁移数据，又不会接管/删除旧项目容器；完整射频验收前保留备份目录、旧镜像
和停止态旧容器。

## Start and verify / 启动与验证 (server / 服务器)

Starting/replacing a container is an explicit server-side step and may interrupt
the live cell. The default project is `gsm-system-live`; the default compose file
is UHD4. 启动或替换容器必须在服务器明确执行，可能中断当前小区；默认项目为
`gsm-system-live`，默认配置为 UHD4。

```bash
cd ~/gsm-system
# UHD4 default: build, compose up -d, then make up to 30 readiness attempts.
# 默认 UHD4：构建、启动，然后最多重试就绪检查 30 次。
./scripts/deploy_to_ubuntu.sh

# Select the project explicitly / 明确选择 Compose 项目：
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live

# Archival Xenial compose (not this clone board) / 归档 Xenial 配置（不适用当前克隆板）：
./scripts/deploy_to_ubuntu.sh --compose-file deploy/docker/docker-compose.yml

# Reuse an existing image or deliberately omit HTTP verification:
# 复用已有镜像，或明确跳过 HTTP 验证：
./scripts/deploy_to_ubuntu.sh --skip-build
./scripts/deploy_to_ubuntu.sh --skip-health
```

The readiness probe polls non-hardware endpoint `/api/v1/cell` with a 30-second
per-request timeout. `HEALTH_RETRIES` and `HEALTH_URL` may be set in the
environment; `--health-url` overrides the URL. Set `GSM_API_TOKEN` to send an
`Authorization: Bearer` header; leave it empty for backward-compatible no-token
deployment. Build, start, and final readiness failures return non-zero.
The script also verifies that this Compose project's containers are running the
expected image IDs before and after HTTP readiness; a different API returning
200 on the host port is not sufficient. 脚本在 HTTP 检查前后验证本项目容器的运行状态
及预期镜像 ID，不以宿主机上其它 API 返回 200 作为发布成功依据。
就绪探针轮询不触发硬件探测的 `/api/v1/cell`，单次超时 30 秒。可通过环境变量设置
重试次数和地址，`--health-url` 覆盖地址；设置 `GSM_API_TOKEN` 会发送 Bearer 头，
留空则保持旧版无令牌行为。构建、启动或最终就绪检查失败均返回非零。

## Full verification / 完整验证 (on server / 服务器)

```bash
curl -fsS --max-time 30 http://127.0.0.1:8082/api/v1/health; echo # uhd_b210 true when SDR is idle
docker exec gsmsystem-uhd4 timeout 90 uhd_usrp_probe 2>&1 | grep -E 'loopback|Error|compat'
# RF full chain 全链路：cell start → ue → sms → voice (see QUICKSTART), then stop.
```

Run the exclusive `uhd_usrp_probe` only while the cell/transceiver is stopped.
When the live transceiver owns the SDR, `uhd_b210: false` or a busy probe does not
prove that hardware is absent. 独占式 `uhd_usrp_probe` 仅在小区/收发器停止时执行；
在线收发器占用 SDR 时出现 `uhd_b210: false` 或 busy，不代表设备不存在。

## Offline script tests / 离线脚本测试

These tests replace `git`, `scp`, `ssh`, `docker`, `curl`, and `sleep` with local
fakes. They require neither network nor Docker and never contact `vm-sdr`.
测试会用本地假命令替换上述工具，无需网络或 Docker，也不会连接 `vm-sdr`。

```powershell
pwsh -NoProfile -File .\scripts\tests\deploy_from_windows.Tests.ps1
```

```bash
sh ./scripts/tests/test_deploy_to_ubuntu.sh
```

## Rollback / 回滚

Prefer the retained same-UHD4 container and image; do not substitute the Xenial
image on this Artix-7 clone board. 首选保留的同版 UHD4 旧容器/镜像；Artix-7 克隆板
不可用 Xenial 镜像作为回滚。

```bash
docker compose -p gsm-system-live -f deploy/docker/docker-compose.uhd4.yml down
docker rename "gsmsystem-uhd4-pre-$stamp" gsmsystem-uhd4
docker start gsmsystem-uhd4
i=0
until curl -fsS --max-time 2 http://127.0.0.1:8082/api/v1/cell >/dev/null; do
  i=$((i + 1)); [ "$i" -lt 30 ] || exit 1; sleep 1
done
```

The legacy entrypoint can overwrite at least Asterisk `sqlite3.db` when it starts,
so starting the old container alone is **not** a data rollback. After its API is
up, keep management writes paused and, before any manual RF start, restore all
four verified snapshots and check them again. 旧入口启动时至少会覆盖 Asterisk
`sqlite3.db`，所以仅启动旧容器不等于数据回滚。旧 API 启动后继续暂停管理写入，手工
启动射频前恢复并复检四份快照：

```bash
docker exec gsmsystem-uhd4 sh -ceu '
backup=/data/backups/pre-persistence-$1
items="OpenBTS/OpenBTS.db:/etc/OpenBTS/OpenBTS.db
OpenBTS/sipauthserve.db:/etc/OpenBTS/sipauthserve.db
OpenBTS/smqueue.db:/etc/OpenBTS/smqueue.db
asterisk/sqlite3.db:/var/lib/asterisk/sqlite3dir/sqlite3.db"
for item in $items; do
  rel=${item%%:*}; dst=${item#*:}; src=$backup/$rel
  [ -f "$src" ] || { echo "missing rollback backup: $src" >&2; exit 1; }
  sqlite3 "$dst" ".restore '\''$src'\''"
  [ "$(sqlite3 "$dst" "PRAGMA quick_check;")" = ok ]
done
' sh "$stamp"
```

Never add `-v` to `compose down`; it would delete project-managed volumes. The
explicit `docker_gsm-data` volume, timestamped backups, old UHD4 image, and old
container must remain until RF chain acceptance. `compose down` 禁止添加 `-v`；完成
射频全链路验收前，必须保留数据卷、时间戳备份、旧 UHD4 镜像和旧容器。

`gsmsystem-dep:2.0` remains an archival Xenial fallback for a genuine Spartan-6
B210 only; it cannot drive the current clone board. `gsmsystem-dep:2.0` 仅供原装
Spartan-6 B210 的归档回滚，不能驱动当前克隆板。
