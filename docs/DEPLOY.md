# Deploy 2.1 / 部署 2.1

> Development machines edit, test Go, and synchronize committed source only.
> Docker builds, container changes, SDR probes, and RF acceptance run on the SDR
> host. 开发机仅编辑、Go 检查与同步；镜像、容器、SDR 与射频操作只在服务器执行。

Release 2.1 uses exactly:

- `deploy/docker/Dockerfile`;
- `deploy/docker/docker-compose.yml`;
- Compose project `gsm-system-live`;
- Compose service `gsm-system`, container `gsmsystem-uhd4`;
- external volume `docker_gsm-data`;
- immutable image `gsm-system:2.1.0-<12sha>` and validated release alias
  `gsm-system:2.1.0`.

The former `.uhd4` Docker/Compose definitions are retired. There is no Xenial
production fallback for the Artix-7-compatible board. 旧 `.uhd4` 构建文件不再使用。

## 1. Preconditions / 前置检查

On `HOST`:

```bash
docker --version
docker compose version
lsusb | grep 2500
ss -tlnp | grep 8082 || echo '8082 is free'
ip route
docker network ls
docker volume inspect docker_gsm-data >/dev/null || docker volume create docker_gsm-data
```

The production service uses its own Docker bridge, by default
`172.31.240.0/24`. Confirm that this subnet does not overlap any host, LAN,
VPN, LTE, container, or routed network before `compose up`; set a different
non-overlapping `GSM_BRIDGE_SUBNET` in `.env` when needed. It must also remain
separate from the OpenBTS handset/GPRS pool `192.168.99.0/24`. 生产容器使用
独立 bridge；启动前必须检查与宿主机、LAN、VPN、LTE、其他容器及
GPRS 网段均不重叠。

The operator must have Docker access without interactive sudo. The current
operator policy retains only the active GSM image ID and its two tags, not an
old image or stopped rollback container. The external business-data volume is
still retained. 当前策略只保留在用 GSM 镜像 ID 及两个标签，不保留旧镜像或停止的
回滚容器；外部业务数据卷继续保留。

## 2. Vendor inputs / 上游源码缓存

The multi-stage Dockerfile builds upstream UHD/OpenBTS components from the
pinned, server-side `third_party/` cache. Initialize or verify it on the server:

```bash
cd ~/gsm-system
./scripts/prefetch_vendor.sh
```

The build verifies `third_party/REVISION_MANIFEST.tsv`. Python/Mako is an
upstream UHD builder-stage dependency, not project-owned management runtime.

## 3. Synchronize committed HEAD / 同步已提交 HEAD

From Windows PowerShell:

```powershell
pwsh -NoProfile -File scripts/deploy_from_windows.ps1 -HostAlias vm-sdr
# Optional remote immutable-image build; does not run compose up:
pwsh -NoProfile -File scripts/deploy_from_windows.ps1 -HostAlias vm-sdr -Build `
  -ComposeFile deploy/docker/docker-compose.yml -ProjectName gsm-system-live
```

The script archives committed `HEAD`, uses a unique remote staging directory,
mirrors the tracked tree, preserves operator-owned `third_party/`, `.git`, env,
and release-revision inputs, and records the 12-character revision. Uncommitted
or untracked changes are not deployed. 同步仅包含已提交 HEAD，不会把当前工作区改动当作发布。

## 4. Back up pre-2.1 state / 备份旧状态

Stop the old cell and pause all config/subscriber writes. Back up these as one
operational set before replacing an older container:

1. `/etc/OpenBTS/OpenBTS.db`;
2. `/etc/OpenBTS/sipauthserve.db`;
3. `/etc/OpenBTS/smqueue.db`;
4. `/var/lib/asterisk/sqlite3dir/sqlite3.db`;
5. `/var/log/asterisk/cdr-csv/Master.csv`, if present.

Use SQLite `.backup`, `umask 077`, mode `0600`, and `PRAGMA quick_check` for each
database. Store a timestamped copy below `/data/backups/pre-2.1-STAMP/` and
initialize the matching `/data/state/` files only after all checks succeed.
四份数据库逐一使用 SQLite 在线备份并检查；CDR 同批复制。不得混用不同批次作为回滚集。

Example skeleton (replace `OLD_CONTAINER` only after inspection):

```bash
old=OLD_CONTAINER
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker inspect "$old" --format '{{range .Mounts}}{{println .Name .Destination}}{{end}}'
docker exec "$old" sh -ceu '
  umask 077
  stamp=$1
  backup=/data/backups/pre-2.1-$stamp
  mkdir -p "$backup/OpenBTS" "$backup/asterisk"
  sqlite3 /etc/OpenBTS/OpenBTS.db ".backup $backup/OpenBTS/OpenBTS.db"
  sqlite3 /etc/OpenBTS/sipauthserve.db ".backup $backup/OpenBTS/sipauthserve.db"
  sqlite3 /etc/OpenBTS/smqueue.db ".backup $backup/OpenBTS/smqueue.db"
  sqlite3 /var/lib/asterisk/sqlite3dir/sqlite3.db ".backup $backup/asterisk/sqlite3.db"
  for db in "$backup"/OpenBTS/*.db "$backup"/asterisk/sqlite3.db; do
    [ "$(sqlite3 "$db" "PRAGMA quick_check;")" = ok ]
    chmod 600 "$db"
  done
  if [ -f /var/log/asterisk/cdr-csv/Master.csv ]; then
    mkdir -p "$backup/asterisk/cdr-csv"
    cp /var/log/asterisk/cdr-csv/Master.csv "$backup/asterisk/cdr-csv/"
    chmod 600 "$backup/asterisk/cdr-csv/Master.csv"
  fi
' sh "$stamp"
```

Do not use `docker compose down -v`; the external volume is operator-owned and
must survive project replacement. 禁止将删卷作为普通升级或回滚步骤。

## 5. Build, start, and publish / 构建、启动与发布

After the Windows `-Build` flow, run the no-RF image fixture first, then deploy
the already-built immutable tag:

```bash
cd ~/gsm-system
revision=$(cat .release-revision)
scripts/tests/test_image.sh "gsm-system:2.1.0-$revision"
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live --skip-build
```

The image test has no USB/RF access and checks OCI/Go identity, vendor manifest,
`/dev/log`, native DB/CDR persistence across recreation, and actual Asterisk
ODBC/CDR-module/dialplan loading. 隔离镜像测试不接 USB、不发射。

For a server-side integrated prefetch/build/deploy flow:

```bash
cd ~/gsm-system
./scripts/prefetch_vendor.sh
./scripts/deploy_to_ubuntu.sh \
  --compose-file deploy/docker/docker-compose.yml \
  --project-name gsm-system-live
```

The script resolves `VERSION` and the 12-character source revision, builds the
immutable tag, verifies OCI labels and `gsm-system --version`, verifies that the
Compose containers use the expected image ID, starts the project, and polls the
non-RF `/api/v1/cell` endpoint. Only after build + container + HTTP validation
does it tag the same image ID as `gsm-system:2.1.0`.

脚本先验证不可变镜像、容器与 HTTP，再更新版本别名；失败不会把候选镜像标成已验证版本。

### API authentication / API 鉴权

Copy `.env.example` to the repository-root `.env`. `GSM_API_TOKEN=` disables
Bearer authentication; a non-empty value enables it. The deploy script passes
the file to Compose without `source`/`eval`, and runs its HTTP probe inside the
new container so a token configured only in `.env` works without a shell export.
Never commit `.env` or print its value.

将 `.env.example` 复制为项目根 `.env`。`GSM_API_TOKEN=` 留空关闭 Bearer 鉴权，
填写非空值则启用。部署脚本不 `source`/`eval` 该文件，而在新容器内执行 HTTP
探针，因此只配置 `.env` 也能正确验收；不要提交 `.env` 或打印令牌值。

`GSM_BIND_ADDRESS` selects the host address for the sole published port,
TCP 8082. Its example value `0.0.0.0` listens on every host interface; use the
SDR host's LAN address to bind only that interface. Do not publish SIP 5060,
OpenBTS 5062, smqueue 5063, sipauthserve 5064, CLI 49300, TRX 5700, or either
RTP range: these native components communicate inside one container. Continue
to enforce the intended LAN access policy in the host firewall; do not disable
or replace Docker's global firewall rules. `GSM_BIND_ADDRESS` 只控制管理面
TCP 8082 的宿主机绑定；所有 SIP/RTP/CLI/TRX 端口仍仅在容器内使用。

### Host-network profile migration / host 网络存档迁移

The `network` field remains required, but under bridge networking it names an
interface **inside** the GSM container. With this single-network Compose file,
that interface is `eth0`. Before recreating an older host-network deployment,
keep the cell stopped and atomically change `/data/last_start.json` from a host
NIC such as `ens33` to `eth0`. Update every saved user preset the same way and
record the before/after configuration diff. Do not start RF merely to rewrite
or validate the file; the migration changes only `network`.

`network` 仍为必填字段，但现在表示 GSM 容器内的出口网卡；本 Compose
的固定值为 `eth0`。从 host 网络升级前，保持小区停止，将
`/data/last_start.json` 和所有用户预设中的宿主机网卡名原子替换为
`eth0`，记录修改前后配置差异；仅修改 `network`，迁移和检查过程不得
启动射频。

After changing only the token, recreate the service container without rebuilding
the image / 仅修改令牌后重建服务容器即可，无需重构镜像：

```bash
docker compose --env-file .env -p gsm-system-live \
  -f deploy/docker/docker-compose.yml \
  up -d --no-build --force-recreate gsm-system
```

`--skip-build` selects an already-built explicitly supplied `GSM_IMAGE`;
`--skip-health` deliberately omits HTTP validation and therefore does not
publish the moving release alias. 跳过健康检查不构成发布成功。

## 6. Acceptance / 验收顺序

1. Development: `go test ./...`, `go vet ./...`, Linux cross-build.
2. Server: isolated image smoke test with no USB, privilege, network, or RF.
3. Verify image labels/version, Compose image ID, `/health`, `/cell`, `/profile`;
   confirm only host TCP 8082 is published and the container has `eth0`.
4. Confirm `/data/state` databases and `/data/log` ownership/modes.
5. Start one permitted single-ARFCN cell; check process state without treating
   `ready` as RF acceptance.
6. Attach dedicated test SIMs; compare `/connections` and `/subscribers`.
7. Bind a unique test number; submit safe-ASCII SMS and independently confirm
   receipt; place a two-way call and inspect real CDR history.
8. Stop the cell; recreate the container; verify profile/database/log/CDR
   persistence and that no cell auto-transmits.

Record exact image ID/tag, revision, timestamps, API request IDs, and native
logs. Local tests or a successful HTTP probe alone do not prove RF readiness.
The management plane was deployed and its non-RF acceptance completed on
2026-09-08. Continue to distinguish that result from the pending handset/RF
acceptance. 管理面已部署并完成非射频验收；真机与射频验收仍须单独记录。

The recorded deployment used runtime revision `16986725daa9`; a later
deployment-script orphan fix `8d361a0` was rerun successfully without changing
the runtime image identity. Exact evidence is in [`RELEASE-2.1.md`](RELEASE-2.1.md).

Offline build/deploy contract checks / 离线构建与部署契约检查：

```bash
sh deploy/docker/test-persistent-state.sh
sh scripts/tests/test_prefetch_vendor.sh
sh scripts/tests/test_deploy_to_ubuntu.sh
sh scripts/tests/test_build_contract.sh
```

```powershell
pwsh -NoProfile -File scripts/tests/deploy_from_windows.Tests.ps1
```

## 7. Logs and persistence / 日志与持久化

- `/data/last_start.json`: `0600`, atomic write + fsync;
- `/data/log/smqueue.log`, `openbts-syslog.log`, `system.log`: `0600`, 16 MiB,
  one `.1` backup via the Go `/dev/log` collector;
- `/data/log/openbts.log`: OpenBTS startup stdout/readiness evidence;
- `/data/log/asterisk/cdr-csv/Master.csv`: persisted Asterisk CDR path;
- `/data/state/OpenBTS/` and `/data/state/asterisk/`: native databases.

The collector owns `/dev/log` only when safe; it never overwrites an active
socket or regular file. 日志接收器仅在安全时创建 `/dev/log`，不会覆盖现有端点。

## 8. Current-version recovery / 当前版本恢复

No previous GSM image or stopped rollback container is retained on the server.
Recover by synchronizing the intended current source revision and redeploying
the current version; do not select an `OLD12SHA` tag. Restore business data only
from an explicitly selected compatible backup, without deleting the external
`docker_gsm-data` volume.

服务器不再保留旧 GSM 镜像或停止的回滚容器。恢复时同步指定的当前源码版本并重新部署，
不要选择 `OLD12SHA`；仅在明确选定兼容备份后恢复业务数据，且不得删除外部
`docker_gsm-data` 卷。
