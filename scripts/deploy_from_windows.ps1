#Requires -Version 7
<# deploy_from_windows.ps1 — sync committed HEAD from Windows to the SDR host.
Usage 用法:
  .\scripts\deploy_from_windows.ps1 [-HostAlias vm-sdr] [-Build]
    [-ComposeFile deploy/docker/docker-compose.yml] [-ProjectName gsm-system-live]

Default only syncs the committed HEAD. -Build builds the selected image remotely,
but never runs `compose up` or replaces a container.
默认只同步已提交的 HEAD；-Build 仅构建镜像，不启动或替换容器。
#>
param(
  [string]$HostAlias = "vm-sdr",
  [switch]$Build,
  [string]$ComposeFile = "deploy/docker/docker-compose.yml",
  [string]$ProjectName = "gsm-system-live"
)

$ErrorActionPreference = "Stop"

function Invoke-CheckedNative {
  param(
    [Parameter(Mandatory)][string]$Command,
    [Parameter(Mandatory)][string[]]$Arguments,
    [Parameter(Mandatory)][string]$Action
  )

  & $Command @Arguments
  $exitCode = $LASTEXITCODE
  if ($exitCode -ne 0) {
    throw "$Action failed / $Action 失败 (exit code $exitCode)"
  }
}

if ($HostAlias -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') {
  throw "HostAlias must be an SSH host name or config alias / HostAlias 必须是 SSH 主机名或配置别名"
}

if ([System.IO.Path]::IsPathRooted($ComposeFile) -or
    $ComposeFile -match '(^|[\\/])\.\.([\\/]|$)' -or
    $ComposeFile -notmatch '^[A-Za-z0-9._/-]+$') {
  throw "ComposeFile must be a safe repository-relative path / ComposeFile 必须是安全的仓库相对路径"
}
$ComposeFile = $ComposeFile.Replace('\', '/')
if ($ProjectName -notmatch '^[a-z0-9][a-z0-9_-]*$') {
  throw "ProjectName must be a safe Compose project name / ProjectName 必须是安全的 Compose 项目名"
}

$root = Split-Path -Parent $PSScriptRoot
$version = (Get-Content -Raw -LiteralPath (Join-Path $root "VERSION")).Trim()
if ($version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+$') {
  throw "VERSION must be semantic x.y.z / VERSION 必须为 x.y.z"
}
$archiveId = [Guid]::NewGuid().ToString("N")
$tgz = Join-Path ([System.IO.Path]::GetTempPath()) "gsm-system-$archiveId.tgz"
$remoteArchive = ".gsm-system-$archiveId.tgz"
$remoteStage = ".gsm-system-stage-$archiveId"

Push-Location $root
try {
  $dirty = @(Invoke-CheckedNative -Command "git" `
    -Arguments @("status", "--porcelain=v1", "--untracked-files=normal") `
    -Action "Inspect working tree / 检查工作区")
  if ($dirty.Count -gt 0) {
    Write-Warning ("Working-tree changes are excluded; syncing committed HEAD only. " +
      "未提交及未跟踪文件不会同步：`n" + ($dirty -join "`n"))
  }

  $head = @(Invoke-CheckedNative -Command "git" `
    -Arguments @("rev-parse", "--short=12", "HEAD") `
    -Action "Resolve HEAD / 读取 HEAD")
  $revision = ($head -join '').Trim()
  if ($revision -notmatch '^[0-9a-f]{12}$') {
    throw "Git revision must be 12 lowercase hexadecimal characters / Git revision 必须为 12 位小写十六进制"
  }
  $immutableImage = "gsm-system:$version-$revision"
  Invoke-CheckedNative -Command "git" `
    -Arguments @("archive", "--format=tar.gz", "-o", $tgz, "HEAD") `
    -Action "Create archive / 创建归档"

  Invoke-CheckedNative -Command "scp" `
    -Arguments @("-o", "BatchMode=yes", $tgz, "${HostAlias}:~/$remoteArchive") `
    -Action "Upload archive / 上传归档"

  # Extract into a unique staging directory, then mirror the exact HEAD while
  # retaining only operator-owned inputs (vendor cache, env and local .git).
  $syncCommand = @'
set -eu
archive="$HOME/__ARCHIVE__"
stage="$HOME/__STAGE__"
cleanup() { rm -rf "$stage" "$archive"; }
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
mkdir -p "$stage" "$HOME/gsm-system"
tar -xzf "$archive" -C "$stage"
rsync -a --delete \
  --exclude=/third_party/ --exclude=/.git/ \
  --exclude=/.env --exclude=/.env.* --exclude=/.release-revision \
  "$stage/" "$HOME/gsm-system/"
printf '%s\n' '__REVISION__' >"$HOME/gsm-system/.release-revision.tmp"
mv "$HOME/gsm-system/.release-revision.tmp" "$HOME/gsm-system/.release-revision"
echo SYNCED
'@.Replace("__ARCHIVE__", $remoteArchive).Replace("__STAGE__", $remoteStage).Replace("__REVISION__", $revision).Replace("`r", "")

  Invoke-CheckedNative -Command "ssh" `
    -Arguments @("-o", "BatchMode=yes", $HostAlias, $syncCommand) `
    -Action "Install archive / 安装归档"
  Write-Host "Synced HEAD $revision with third_party preserved / 已同步 HEAD，保留 third_party"

  if ($Build) {
    $buildCommand = @'
set -eu
cd "$HOME/gsm-system"
export GSM_VERSION='__VERSION__' GSM_REVISION='__REVISION__' GSM_IMAGE='__IMAGE__'
docker compose -p '__PROJECT__' -f '__COMPOSE__' build
test "$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.version" }}' "$GSM_IMAGE")" = "$GSM_VERSION"
test "$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "$GSM_IMAGE")" = "$GSM_REVISION"
docker run --rm --entrypoint /usr/local/bin/gsm-system "$GSM_IMAGE" --version
'@.Replace("__VERSION__", $version).Replace("__REVISION__", $revision).Replace("__IMAGE__", $immutableImage).Replace("__PROJECT__", $ProjectName).Replace("__COMPOSE__", $ComposeFile).Replace("`r", "").Replace("`n", "; ")
    Invoke-CheckedNative -Command "ssh" `
      -Arguments @("-o", "BatchMode=yes", $HostAlias, $buildCommand) `
      -Action "Remote compose build / 远端 Compose 构建"
    Write-Host "Build complete; no container was started / 构建完成，未启动容器"
  }
} finally {
  Pop-Location
  Remove-Item -LiteralPath $tgz -Force -ErrorAction SilentlyContinue
}
