# AGENTS.md — gsm-system dev rules 开发规范 (bilingual 双语)

> Dev machine 开发机 only edits code/docs/git; **never runs RF, docker, resident
> services 不跑射频/docker/常驻服务**. All image builds, cell bring-up, UE/SMS/voice
> verification run on the SDR server (`vm-sdr`). 一切构建/拉起/验证都在SDR服务器上执行。

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
- API change must sync API改必同步 `docs/API.md` + `docs/api/openapi.yaml` + `*_test.go`.
- New endpoints only under `/api/v1` (envelope+status); root legacy frozen.
- Secrets out of git: real IMSIs/Ki/volumes/IPs → `.example` only.
-瘦身：never commit `*.log/*.pcap/bin/__pycache__/crash/*_run.conf`; samples only `docs/samples/`.

## 5. Submit & release 提交与发布

- Message 信息：`<scope>: <what> / <中文说明>` (e.g. `api: freeze legacy sendsms / 冻结旧sendsms语义`), one thing per commit.
- Before push 推前必跑：`go test ./...` green 全绿 + `go vet ./...` + `git status` clean.
- Release via `scripts/deploy_from_windows.ps1` (sync) then server `docker compose up -d --build`;
  rollback = previous image tag + kept old container `gsmsystem`.
- Docs and commits are bilingual 文档与提交均为中英双语. GitHub repo is private 仓库私有。
