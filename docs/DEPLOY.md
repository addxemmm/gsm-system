# Deploy 2.1 / 部署 2.1

On hosts shared with other projects, use `--skip-cleanup` with the deployment
script to retain all image identity, HTTP and binary-health gates while skipping
automatic cleanup. Then remove only explicitly verified obsolete GSM objects;
do not run a global builder prune without confirming its cross-project impact.
This option does not skip health checks or remove data volumes.
共享主机部署可加 `--skip-cleanup`：保留镜像身份、HTTP 与二进制健康门禁，仅跳过自动清理。
随后按精确 ID 清理已核实的旧 GSM 对象；未确认跨项目影响前，不执行全局构建缓存清理。
此选项不跳过健康检查，也不删除数据卷。

> Development machines edit, test Go, and synchronize committed source only.
> Docker builds, container changes, SDR probes, and RF acceptance run on the SDR
> host. 开发机仅编辑、Go 检查与同步；镜像、容器、SDR 与射频操作只在服务器执行。

Release 2.1 uses exactly:

- `deploy/docker/Dockerfile`;
- `deploy/docker/docker-compose.yml`;
- optional `deploy/docker/docker-compose.api.yml` for explicit API publishing / 可选独立 API 端口 overlay;
- Compose project `gsm-system-live`;
- Compose service `gsm-system`, container `gsmsystem-uhd4`;
- external volume `docker_gsm-data`;
- the single runtime image tag `gsm-system:2.1`; application/OCI version remains
  `2.1.0`, and the exact source revision remains in OCI/binary metadata.

The former `.uhd4` Docker/Compose definitions are retired. There is no Xenial
production fallback for the Artix-7-compatible board. 旧 `.uhd4` 构建文件不再使用。

### Manual container startup / 容器仅手动启动

Compose sets `restart: "no"`: the GSM container does not start automatically
after host/Docker restart or automatically restart after exit. Docker itself and
unrelated containers keep their existing policies. An explicit deployment still
starts the management container; it does not automatically start the radio cell.
容器不随开机或 Docker 重启启动，退出后也不自动重启；Docker 服务和其他容器策略不变。
显式执行部署脚本仍会启动管理容器，但不自动启动射频小区。

For an existing container, change only the restart policy without interrupting
its current run, then verify it / 已有容器可无中断修改并查询策略：

```bash
docker update --restart=no gsmsystem-uhd4
docker inspect --format '{{.HostConfig.RestartPolicy.Name}}' gsmsystem-uhd4
# Expected / 预期: no
```

After a reboot, manually start the existing container when needed:
`docker start gsmsystem-uhd4`. Then follow [QUICKSTART.md](QUICKSTART.md) to inspect
state, reapply stopped-state NAT if needed, and explicitly start the cell via API.
For new code/configuration deployment, use the gated release script instead.
重启后需要使用时手动启动已有容器，再检查状态、按需恢复停止态 NAT，并通过 API 显式
启动小区；发布新代码或配置仍使用带验收门禁的部署脚本。

## 1. Preconditions / 前置检查

The default published port is Web `18082`; the independent API is unpublished
and loopback-bound inside the container. Set `GSM_EXPOSE_API=true` in root `.env`
to let this deployment script also publish API `8082`. Host ports can be changed
with `GSM_WEB_PORT` and `GSM_API_PORT`. See [WEB-CONSOLE.md](WEB-CONSOLE.md).
默认仅发布 Web 18082；独立 API 仅容器内回环。根 `.env` 中开启上述开关后，部署脚本
会合并 API overlay 并发布 8082；两宿主机端口均可配置。测试环境可显式同时开放。

On `HOST`:

```bash
docker --version
docker compose version
lsusb | grep 2500
ss -tlnp | grep -E ':18082[[:space:]]' || echo '18082 is free'
# Only for explicit standalone API exposure / 仅显式开放独立 API 时检查
ss -tlnp | grep -E ':8082[[:space:]]' || echo '8082 is free'
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
operator policy retains only the active `gsm-system:2.1` image and no stopped
rollback container. Every healthy deployment removes superseded GSM tags/images,
stopped managed GSM containers, and unused Docker build cache. LTE containers,
LTE images, and the external business-data volume remain untouched. 当前策略仅保留
在用 `gsm-system:2.1` 镜像；每次健康验收后清理旧 GSM 对象及无用构建缓存，LTE
容器/镜像和外部业务数据卷保持不变。

## 2. Startup configuration and vendor inputs / 启动配置与上游源码缓存

### Project timezone / 项目时区

Set `TZ=Asia/Shanghai` in the root `.env` (the default, UTC+08:00), or another
installed IANA name such as `Asia/Hong_Kong` or `UTC`, before creating the
container. The same setting controls container `date`, Go/native process logs
and API timestamp display. Invalid names fail startup explicitly. This changes
timezone presentation, not the host/kernel clock; do not manually add eight
hours to system time. The standalone Go YAML setting is `timezone`; a nonempty
`TZ` environment variable takes precedence. Recreate the container to apply a
changed deployment timezone. CDR files retain UTC storage and are converted
only when queried.
根 `.env` 使用 `TZ=Asia/Shanghai` 默认东八区，也可改为上述其他 IANA 名称。
它统一容器、Go/原生日志及 API 展示；非法值明确启动失败，仅改变时区展示，
不改宿主机时钟、不手动加八小时。独立 Go 可配置 YAML `timezone`，非空环境变量
`TZ` 优先；部署修改后重建容器生效，话单仍以 UTC 存储、查询时转换。

SMS queries are limited to the latest cell start recognized by this management
process. After container/API recreation, `GET /api/v1/sms` returns an empty
current-start view until a new cell start establishes a boundary. Persistent
logs and subscriber bindings remain intact; historical runs are not relabeled
using the new timezone. / 短信查询仅限本管理进程识别的最近小区启动；重建后尚未
启动时返回空范围，不删除日志或绑定，也不拿新时区重解释旧轮日志。

Fresh data volumes seed `GPRS.Enable=1`, two C0 packet-data channels and
`GGSN.DNS=${GSM_GPRS_DNS:-1.1.1.1}`. The DNS must be a reachable upstream IPv4
resolver, not a container/host loopback stub. Existing OpenBTS database settings,
including an explicitly disabled GPRS service, are preserved on recreation.
This initializes packet-data configuration; it does not start the cell or prove
handset Internet connectivity. See [SIM and GPRS acceptance](SIM.md).
新数据卷默认开启 GPRS、设置两个 C0 分组信道，并使用上述 DNS 环境参数；DNS 须是
可达上游 IPv4 地址，不使用回环解析器。重建保留旧卷设置（包括主动关闭 GPRS）。
默认配置不会自动启动小区，亦不代表真机已能上网，验收步骤见链接。

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
# Optional remote gsm-system:2.1 build; does not run compose up:
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
the already-built stable runtime tag:

```bash
cd ~/gsm-system
scripts/tests/test_image.sh gsm-system:2.1
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

The script resolves `VERSION` and the 12-character source revision and builds
only `gsm-system:2.1`. Before deployment it verifies the OCI version/revision
labels and `gsm-system --version`; it then verifies the Compose image ID, polls
the non-RF `/api/v1/cell` endpoint, and runs the state-aware binary health probe.
Only after every gate passes does it delete stopped managed GSM containers,
superseded GSM tags/images, and unused build cache. It never runs `system prune`
or removes a volume, network, LTE container, or LTE image.

脚本仅构建 `gsm-system:2.1`，启动前核对 OCI/二进制 revision；容器、HTTP 与状态感知
健康探针全部通过后，才清理旧 GSM 对象及无用构建缓存，且不删除卷、网络或 LTE 对象。

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

For token/timezone-only changes, first stop the cell using its current API
credentials, then edit root `.env`. Apply the changed Compose environment through
the same release/health/cleanup gates without rebuilding the image. Retain the
`.release-revision` matching that image; do not replace it with a newer docs-only
HEAD. / 仅修改令牌或时区时，先用当前凭据停止小区，再编辑根 `.env`；仍经发布门禁
应用配置，无需构建镜像，保留与镜像匹配的发布标记，不改成较新的纯文档 HEAD。

```bash
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live --skip-build
```

`--skip-build` selects the already-built `gsm-system:2.1` image and still checks
its recorded revision before deployment. `--skip-health` deliberately omits
health validation and therefore also skips all cleanup. 跳过健康检查不构成发布成功，
也不会执行清理。

Compose recreates the service when the effective environment changes; an
unchanged invocation need not recreate it. Each later explicit cell start checks
and restores missing NAT before radio launch. Stopped-state `PUT /api/v1/network`
with `{"iface":"eth0"}` remains available for independent verification.
A docs-only update requires neither an image build nor a container restart:
sync only documentation if needed. Full `deploy_from_windows.ps1` sync records
HEAD as `.release-revision`, so follow a full sync with a matching build rather
than editing labels to pass the mismatch check.
有效环境变化时 Compose 会重建容器，配置未变则未必重建；后续显式启动在拉起射频前
自动补齐 NAT，停止态也可独立检查。纯文档更新不需要构建/重启，可仅同步文档；全量同步会记录 HEAD，
因此全量同步后应构建对应镜像，不伪改标签绕过版本核对。

Compose uses the binary's read-only `--healthcheck`: a stopped cell is healthy,
and a running cell is healthy only when `ready=true`; `transitioning` and
`degraded` are unhealthy. The probe honors `GSM_API_TOKEN`, starts no native
process, and is management/process-state evidence rather than RF or handset
acceptance. / Compose 使用只读 `--healthcheck`：小区停止时健康，运行时仅
`ready=true` 才健康；`transitioning` 与 `degraded` 均不健康。探针遵循令牌配置且
不启动原生进程，只证明管理面与进程状态，不代表射频或真机业务验收。

## 6. Acceptance / 验收顺序

1. Development: `go test ./...`, `go vet ./...`, Linux cross-build.
2. Server: all four isolated image suites below, with no production data or RF.
3. Verify image labels/version, Compose image ID, `/health`, `/cell`, `/profile`;
   confirm only Web TCP 18082 is published by default (or both Web/API when
   `GSM_EXPOSE_API=true`), and the container has `eth0`.
   默认仅 Web 端口；显式开启独立 API 时检查两端口，原生服务端口不发布。
4. Confirm `/data/state` databases and `/data/log` ownership/modes.
5. Start one permitted single-ARFCN cell; check process state without treating
   `ready` as RF acceptance.
6. Attach dedicated test SIMs; compare `/connections` and `/subscribers`.
7. Bind a unique test number; test 2600/2602, short Chinese handset SMS in both
   directions, safe-ASCII API submission and independent receipt timing; then
   place a two-way call and inspect real CDR history.
8. Stop the cell; recreate the container; verify profile/database/log/CDR
   persistence and that no cell auto-transmits.

Record exact image ID/tag (`gsm-system:2.1`), revision, timestamps, API request IDs, and native
logs. Local tests or a successful HTTP probe alone do not prove RF readiness.
The management plane was deployed and its non-RF acceptance completed on
2026-09-08. Continue to distinguish that result from the pending handset/RF
acceptance. 管理面已部署并完成非射频验收；真机与射频验收仍须单独记录。

For the latest deployed runtime revision and acceptance evidence, see
[`RELEASE-2.1.md`](RELEASE-2.1.md). The saved uplink is `eth0`; business data
and LTE state must remain unchanged across deployment and cleanup.

Welcome-default migration and the state-aware container probe have offline
coverage, but handset SMS transmit/receive and packet-data/DNS/Internet remain
pending end-to-end acceptance. / 欢迎默认迁移与状态感知容器探针已有离线覆盖；
手机短信收发及分组数据/DNS/互联网仍待端到端验收。

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

Final-image checks on the SDR server / SDR 服务器上的最终镜像验收：

```bash
for suite in test_image test_callerid_image test_sms_image test_presets_image; do
  sh "scripts/tests/${suite}.sh" gsm-system:2.1 || exit 1
done
```

These suites clean their own isolated containers/volumes. Native build gates
also exercise GMM parsing, UHD timeout recovery and UCS-2 decoding against
production methods, including original-source negative controls where provided.
Record tests and handset acceptance separately; see [OPERATIONS.md](OPERATIONS.md).
各套件自行清理隔离资源；原生构建另验收 GMM、UHD 和 UCS-2，含已提供的旧源码负向
对照。离线测试与真机验收分开记录，不能互相代替。

## 7. Logs and persistence / 日志与持久化

- `/data/last_start.json`: `0600`, atomic write + fsync;
- `/data/log/smqueue.log`, `openbts-syslog.log`, `system.log`: `0600`, 16 MiB,
  one `.1` backup via the Go `/dev/log` collector;
- `/data/log/openbts.log`: OpenBTS stdout, 16 MiB plus one `.1` backup; append on restart, with readiness detected only from the current process output. / OpenBTS 输出按 16 MiB 加一份备份轮转，重启保留证据，就绪仅取本次进程输出；
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
