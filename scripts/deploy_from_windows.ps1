#Requires -Version 7
<# deploy_from_windows.ps1 — Windows local syncs code to Ubuntu server, build there.
Usage 用法: .\scripts\deploy_from_windows.ps1 [-HostAlias vm-sdr] [-Build]
  Default only syncs 默认只同步；-Build also runs remote compose build.
Prereq 前置: ~/.ssh/config has Host vm-sdr (key login).
Never rebuilds containers here (avoids interrupting live cell).
#>
param([string]$HostAlias = "vm-sdr", [switch]$Build)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$tgz = Join-Path ([System.IO.Path]::GetTempPath()) "gsm-system.tgz"

Push-Location $root
try {
  # git archive (not tar .) — OneDrive file locks break bsdtar reads;
  # archive comes from committed HEAD, so commit before syncing.
  git archive --format=tar.gz -o $tgz HEAD | Out-Null
  scp -o BatchMode=yes $tgz "${HostAlias}:~/gsm-system.tgz"
  ssh -o BatchMode=yes $HostAlias "mkdir -p ~/gsm-system && chmod -R u+rwx ~/gsm-system ; tar -xzf ~/gsm-system.tgz -C ~/gsm-system && echo SYNCED"
  if ($Build) {
    ssh -o BatchMode=yes $HostAlias "cd ~/gsm-system && docker compose -f deploy/docker/docker-compose.yml build 2>&1 | tail -5"
  }
} finally {
  Pop-Location
  Remove-Item $tgz -ErrorAction SilentlyContinue
}
