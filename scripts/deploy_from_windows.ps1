#Requires -Version 7
<# deploy_from_windows.ps1 — Windows local syncs code to Ubuntu server, build there.
Usage 用法: .\scripts\deploy_from_windows.ps1 [-HostAlias vm-sdr] [-Build]
  Default only syncs 默认只同步；-Build also runs remote compose build.
Prereq 前置: ~/.ssh/config has Host vm-sdr (key login).
Never rebuilds containers here (avoids interrupting live cell).
#>
param([string]$HostAlias = "vm-sdr", [switch]$Build)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$tgz = Join-Path ([System.IO.Path]::GetTempPath()) "gsm-system.tgz"

Push-Location $root
try {
  tar --exclude=.git --exclude=bin --exclude=var -czf $tgz . | Out-Null
  scp -o BatchMode=yes $tgz "${HostAlias}:~/gsm-system.tgz"
  ssh -o BatchMode=yes $HostAlias "mkdir -p ~/gsm-system && tar -xzf ~/gsm-system.tgz -C ~/gsm-system && echo SYNCED"
  if ($Build) {
    ssh -o BatchMode=yes $HostAlias "cd ~/gsm-system && docker compose -f deploy/docker/docker-compose.yml build 2>&1 | tail -5"
  }
} finally {
  Pop-Location
  Remove-Item $tgz -ErrorAction SilentlyContinue
}
