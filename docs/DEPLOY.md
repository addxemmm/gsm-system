# Deploy 2.1 / 部署 2.1

> Development machines edit, test Go, and synchronize committed source only.
> Docker builds, container changes, SDR probes, and RF acceptance run on the SDR
> host. 开发机仅编辑、Go 检查与同步；镜像、容器、SDR 与射频操作只在服务器执行。

Release 2.1 uses exactly:

- `deploy/docker/Dockerfile`;
- `deploy/docker/docker-compose.yml`;
- Compose project `gsm-system-live`;
- Compose service `gsm-system`, container `gsmsystem-uhd4` (leaves the stopped
  rollback container `gsmsystem` name available);
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
docker volume inspect docker_gsm-data >/dev/null || docker volume create docker_gsm-data
```

The operator must have Docker access without interactive sudo. Keep a verified
old immutable image, a stopped rollback container, and state backups until full
RF acceptance. Docker Hub/network failures may be retried after connectivity is
restored; cached native build layers should be retained.

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
When `GSM_API_TOKEN` is configured, export it on the host so the health probe can
send the Bearer token. Do not commit it.

`--skip-build` selects an already-built explicitly supplied `GSM_IMAGE`;
`--skip-health` deliberately omits HTTP validation and therefore does not
publish the moving release alias. 跳过健康检查不构成发布成功。

## 6. Acceptance / 验收顺序

1. Development: `go test ./...`, `go vet ./...`, Linux cross-build.
2. Server: isolated image smoke test with no USB, privilege, network, or RF.
3. Verify image labels/version, Compose image ID, `/health`, `/cell`, `/profile`.
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
当前 2.1 仍为候选版，服务器构建和真机结果完成前不得写成“已发布/已验收”。

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

## 8. Rollback / 回滚

Choose a retained immutable tag and deploy without rebuilding:

```bash
export GSM_IMAGE=gsm-system:2.1.0-OLD12SHA
export GSM_REVISION=OLD12SHA
./scripts/deploy_to_ubuntu.sh --project-name gsm-system-live --skip-build
```

If native data changed after cutover, stop writes and restore the matching
timestamped database/CDR set before RF restart. Revalidate image ID, HTTP,
database quick checks, persistence, and then the RF chain. Restoring an image
without its compatible data set is not a complete rollback. 回滚必须同时考虑镜像与数据集。
