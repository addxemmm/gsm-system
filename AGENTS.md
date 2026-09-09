# AGENTS.md — gsm-system dev rules 开发规范 (bilingual 双语)

> Dev machine 开发机 only edits code/docs/git; **never runs RF, docker, resident
> services 不跑射频/docker/常驻服务**. Image builds and no-RF isolated tests may run
> on the SDR build server or ephemeral GitHub-hosted release runners. Real cell
> bring-up and UE/SMS/voice verification run only on `vm-sdr`.
> 镜像构建及无射频隔离测试可在 SDR 构建服务器或临时 GitHub 发布 runner 执行，
> 真正小区启动和手机业务验证仅在 SDR 服务器上执行，不安装自管 runner 到射频服务器。

## 1. Parallelism 并发与 subagent

- Independent workstreams go parallel via Task (`explore`/`general`); main session merges.
- Subagent prompt must include: workdir, target files, output cap (800–1500 words), deliverable.

## 2. Handoff 交接

- Cross-session handoff = `git log --oneline -10` + this file + `docs/MIGRATION.md`.
- Before interrupt: branch, `git status --short`, open todos, next command (`go test ./...`).

## 3. Folk 分叉试错

- Risky changes (DB reset, kill logic, iptables) go `folk/<name>` first, merge after verify.
- RF-related folk must attach server logs (`docker logs` + `curl`) to merge.

## 4. Iron rules 铁律

- Dev may run 可执行：`go test ./...`, `go vet ./...`, `go build` (incl. `GOOS=linux`),
  docs edit, `git`. May NOT 不可：`docker build/run`, resident `./bin/gsm-system`,
  conclusive `uhd_find_devices` (no hardware here).
- Read→Grep/Glob→Edit(minimal diff)→Bash(`workdir`=repo root, no `cd`).
- API change must sync API改必同步 `docs/API.md` + `docs/api/openapi.yaml` +
  `postman/` + `internal/contract/*_test.go`.
- Public management API endpoints live only under `/api/v1` (envelope + HTTP
  status). The Web listener additionally serves known embedded static assets
  and secret-free `/web-meta.json`; this never restores retired root API routes.
  Unknown/legacy routes remain real `404`, not an SPA fallback. 管理 API 仍只在
  `/api/v1`；Web 静态入口不恢复旧根接口，未知路径必须返回真实 404。
- Secrets out of git: real IMSIs/Ki/volumes/IPs → `.example` only.
-瘦身：never commit `*.log/*.pcap/bin/__pycache__/crash/*_run.conf`; samples only `docs/samples/`.

## 5. Submit & release 提交与发布

- Registry publication is separate from live deployment: reviewed `vX.Y.Z` tags
  trigger `.github/workflows/release.yml` to test/publish full versions and the
  `2.1` alias, with bilingual GitHub Releases. No automatic VERSION bump, `latest`
  tag, server access or RF start. Use dry-run before creating a new immutable tag.
  镜像发版与现网部署分离；版本 tag 驱动云端验收、Hub 上传和双语 Release，默认可先
  dry-run，不自动改 VERSION、不推 latest、不访问服务器或开启射频。

- Message 信息：`<scope>: <what> / <中文说明>` (e.g. `api: retire root routes / 移除根路径旧接口`), one thing per commit.
- Before push 推前必跑：`go test ./...` green 全绿 + `go vet ./...` + `git status` clean.
- Release via `scripts/deploy_from_windows.ps1` (sync), then run
  `scripts/deploy_to_ubuntu.sh` on the server; direct `compose up` bypasses the
  mandatory revision/health/cleanup gates. Keep the release tag at `2.1` until
  the operator explicitly instructs a version change.
  The runtime image tag remains exactly `gsm-system:2.1`; source traceability
  stays in OCI/binary revision metadata. After every healthy deployment, remove
  stopped managed GSM containers, superseded GSM image IDs, and unused Docker
  build cache. Never remove LTE objects or the external business-data volume.
  运行镜像固定为 `gsm-system:2.1`，源码 revision 由 OCI/二进制元数据追踪；每次健康
  验收后清理停止的 GSM 容器、旧 GSM 镜像及无用构建缓存，绝不删除 LTE 对象或业务数据卷。
- Docs and commits are bilingual 文档与提交均为中英双语. Check GitHub repository
  visibility before publication; never assume it is private. 推送前核验仓库可见性，
  不假定仓库私有；密钥、站点环境与业务数据始终禁止提交。
