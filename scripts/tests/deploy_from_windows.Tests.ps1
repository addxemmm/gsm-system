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
  echo abcdef123456
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
rem PowerShell's .cmd adapter splits BatchMode=yes at '='; ssh.exe does not.
if not "%4"=="sh" (
  echo bad ssh argv: [%1][%2][%3][%4][%5][%6] 1>&2
  exit /b 41
)
if not "%5"=="-s" exit /b 42
if not "%6"=="--" exit /b 43
echo SSH_STDIN_BEGIN>>"%FAKE_DEPLOY_LOG%"
more >>"%FAKE_DEPLOY_LOG%"
echo SSH_STDIN_END>>"%FAKE_DEPLOY_LOG%"
if "%FAKE_SSH_FAIL%"=="1" exit /b 44
exit /b 0
'@ | Set-Content -LiteralPath (Join-Path $bin "ssh.cmd") -Encoding ascii

  $oldPath = $env:PATH
  $env:PATH = "$bin;$oldPath"
  $env:FAKE_DEPLOY_LOG = $log
  $env:FAKE_SCP_FAIL = "0"
  $env:FAKE_SSH_FAIL = "0"

  $output = & pwsh -NoProfile -File $script -HostAlias test-sdr -Build 2>&1 | Out-String
  Assert-True ($LASTEXITCODE -eq 0) "happy path should succeed: $output"
  $calls = Get-Content -Raw -LiteralPath $log
  $source = Get-Content -Raw -LiteralPath $script
  Assert-True ($output -match "excluded") "dirty HEAD-only sync warning should be visible"
  Assert-True ($calls -match "gsm-system:2\.1\.0-abcdef123456") "immutable image must use VERSION plus 12-char revision"
  Assert-True ($calls -match "docker compose -p 'gsm-system-live' -f 'deploy/docker/docker-compose\.yml' build") "single production Compose file must be the build default"
  Assert-True ($calls -match "docker compose --env-file .env -p 'gsm-system-live'") "existing root .env must be explicit during build"
  Assert-True ($calls -notmatch "compose .*up") "Windows sync/build must never run compose up"
  $sshCalls = @($calls -split "`r?`n" | Where-Object { $_ -like "ssh *" })
  Assert-True ($sshCalls.Count -eq 2) "sync plus build should invoke two remote programs"
  Assert-True (@($sshCalls | Where-Object { $_ -notmatch ' sh -s --$' }).Count -eq 0) "every remote program must use explicit POSIX sh stdin mode"
  Assert-True ($calls -match "--exclude=/\.env\.\*") "zsh-sensitive rsync glob must arrive through sh stdin without login-shell parsing"
  Assert-True ($source -match "rsync -a --delete") "sync must delete stale tracked files"
  Assert-True ($source -match "--exclude=/third_party/") "sync must preserve third_party"
  Assert-True ($source -match "\.release-revision") "sync must record the archived HEAD revision"
  Assert-True ($source -notmatch "docker-compose\.uhd4") "retired experimental Compose must not be referenced"

  Clear-Content -LiteralPath $log
  $env:FAKE_SCP_FAIL = "1"
  & pwsh -NoProfile -File $script -HostAlias test-sdr *> $null
  Assert-True ($LASTEXITCODE -ne 0) "an scp native failure must fail the script"
  $calls = Get-Content -Raw -LiteralPath $log
  Assert-True ($calls -notmatch "^ssh " -and $calls -notmatch "`nssh ") "ssh must not run after failed scp"

  Clear-Content -LiteralPath $log
  $env:FAKE_SCP_FAIL = "0"
  $env:FAKE_SSH_FAIL = "1"
  & pwsh -NoProfile -File $script -HostAlias test-sdr *> $null
  Assert-True ($LASTEXITCODE -ne 0) "an explicit remote sh failure must fail the script"
  $calls = Get-Content -Raw -LiteralPath $log
  Assert-True ($calls -match '(?m)^ssh .*test-sdr sh -s --\r?$') "remote sh argv must be fixed even when the account login shell is zsh"

  Write-Host "PASS deploy_from_windows.Tests.ps1 / Windows 部署脚本测试通过"
} finally {
  $env:PATH = $oldPath
  Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}
