# Offline contract tests for deploy_from_windows.ps1. No Pester/network required.
# deploy_from_windows.ps1 离线契约测试：无需 Pester、网络或服务器。
#Requires -Version 7
$ErrorActionPreference = "Stop"

function Assert-True([bool]$Condition, [string]$Message) {
  if (-not $Condition) { throw "ASSERT: $Message" }
}

$script = (Resolve-Path (Join-Path $PSScriptRoot "../deploy_from_windows.ps1")).Path
$temp = Join-Path ([IO.Path]::GetTempPath()) "gsm-deploy-test-$([Guid]::NewGuid().ToString('N'))"
$bin = Join-Path $temp "bin"
$log = Join-Path $temp "native.log"
New-Item -ItemType Directory -Path $bin | Out-Null

try {
  @'
@echo off
echo git %*>>"%FAKE_DEPLOY_LOG%"
if "%1"=="status" (
  echo  M local-only.txt
  exit /b 0
)
if "%1"=="rev-parse" (
  echo abc1234
  exit /b 0
)
if "%1"=="archive" (
  :find_output
  shift
  if "%1"=="-o" (
    type nul > "%2"
    exit /b 0
  )
  if "%1"=="" exit /b 8
  goto find_output
  exit /b 0
)
exit /b 9
'@ | Set-Content -LiteralPath (Join-Path $bin "git.cmd") -Encoding ascii
  @'
@echo off
echo scp %*>>"%FAKE_DEPLOY_LOG%"
if "%FAKE_SCP_FAIL%"=="1" exit /b 23
exit /b 0
'@ | Set-Content -LiteralPath (Join-Path $bin "scp.cmd") -Encoding ascii
  @'
@echo off
echo ssh %*>>"%FAKE_DEPLOY_LOG%"
exit /b 0
'@ | Set-Content -LiteralPath (Join-Path $bin "ssh.cmd") -Encoding ascii

  $oldPath = $env:PATH
  $env:PATH = "$bin;$oldPath"
  $env:FAKE_DEPLOY_LOG = $log
  $env:FAKE_SCP_FAIL = "0"

  $output = & pwsh -NoProfile -File $script -HostAlias test-sdr -Build 2>&1 | Out-String
  Assert-True ($LASTEXITCODE -eq 0) "happy path should succeed"
  $calls = Get-Content -Raw -LiteralPath $log
  $source = Get-Content -Raw -LiteralPath $script
  Assert-True ($output -match "excluded") "dirty HEAD-only sync warning should be visible"
  Assert-True ($calls -match "docker compose -f 'deploy/docker/docker-compose\.uhd4\.yml' build") "UHD4 must be the build default"
  Assert-True ($calls -notmatch "compose .*up") "Windows sync/build must never run compose up"
  Assert-True ($source -match "rsync -a --exclude=/third_party/") "sync must overlay HEAD and preserve third_party"

  Clear-Content -LiteralPath $log
  $env:FAKE_SCP_FAIL = "1"
  & pwsh -NoProfile -File $script -HostAlias test-sdr *> $null
  Assert-True ($LASTEXITCODE -ne 0) "an scp native failure must fail the script"
  $calls = Get-Content -Raw -LiteralPath $log
  Assert-True ($calls -notmatch "^ssh " -and $calls -notmatch "`nssh ") "ssh must not run after failed scp"

  Write-Host "PASS deploy_from_windows.Tests.ps1 / Windows 部署脚本测试通过"
} finally {
  $env:PATH = $oldPath
  Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}
