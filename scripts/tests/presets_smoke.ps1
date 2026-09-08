#Requires -Version 7
<#
Exercise preset CRUD against a running management API; never start a cell.
验证预设增删改查；不发送有效的小区启动请求，不开启射频。
Creates one uniquely named fixture and removes it in finally. Token is read
from GSM_API_TOKEN when configured; no credentials are printed.
创建唯一命名测试配置并在 finally 中删除；可从环境变量读取 Token。
#>
param([Parameter(Mandatory)][uri]$BaseUrl)
$ErrorActionPreference = 'Stop'
$base = $BaseUrl.AbsoluteUri.TrimEnd('/')
if ($BaseUrl.Scheme -notin @('http', 'https')) { throw 'HTTP(S) URL required' }
$headers = @{}
if (-not [string]::IsNullOrWhiteSpace($env:GSM_API_TOKEN)) {
    $headers.Authorization = 'Bearer ' + $env:GSM_API_TOKEN.Trim()
}
$id = 'smoke-' + [guid]::NewGuid().ToString('N')
$created = $false

function Request([string]$Method, [string]$Path, [int]$Expected, $Body = $null) {
    $args = @{ Uri = "$base$Path"; Method = $Method; Headers = $headers;
        SkipHttpErrorCheck = $true; TimeoutSec = 30 }
    if ($null -ne $Body) {
        $args.ContentType = 'application/json'
        $args.Body = $Body | ConvertTo-Json -Depth 12 -Compress
    }
    $response = Invoke-WebRequest @args
    if ([int]$response.StatusCode -ne $Expected) {
        throw "$Method $Path returned $($response.StatusCode), expected $Expected"
    }
    $json = $response.Content | ConvertFrom-Json
    if (-not $json.request_id) { throw 'Missing request_id' }
    if ($Expected -lt 300 -and $json.code -ne 0) { throw 'Invalid success envelope' }
    return $json
}

try {
    $before = Request GET '/cell' 200
    $params = @{ arfcns = '1'; c0 = '55'; band = '900'; mcc = '001'; mnc = '01';
        lac = '1'; ci = '1'; short_name = 'LAB'; network = 'eth0' }
    $fixture = @{ id = $id; name = 'Smoke test'; description = 'Temporary / 临时测试'; params = $params }
    $null = Request GET '/presets' 200
    $null = Request POST '/presets' 201 $fixture
    $created = $true
    $null = Request POST '/presets' 409 $fixture
    $loaded = Request GET "/presets/$id" 200
    if ($loaded.data.params.network -ne 'eth0') { throw 'Preset round-trip failed' }
    $params.c0 = '540'; $params.band = '1800'
    $updated = Request PUT "/presets/$id" 200 @{
        name = 'Updated smoke'; description = 'Temporary / 临时测试'; params = $params }
    if ($updated.data.params.band -ne '1800') { throw 'Preset update failed' }
    # Mixed selector/explicit fields must be rejected before any hardware work.
    # 混合选择器与显式参数必须在访问硬件前拒绝。
    $null = Request POST '/cell' 422 @{ preset_id = $id; network = 'eth0' }
    $null = Request POST '/cell' 404 @{ preset_id = "$id-missing" }
    $params.c0 = '55' # Deliberately invalid for 1800 / 故意制造频段信道不匹配。
    $null = Request PUT "/presets/$id" 422 @{ name = 'Invalid'; description = ''; params = $params }
    $loaded = Request GET "/presets/$id" 200
    if ($loaded.data.params.c0 -ne '540') { throw 'Rejected update changed stored preset' }
    $null = Request DELETE "/presets/$id" 200
    $created = $false
    $null = Request GET "/presets/$id" 404
    $after = Request GET '/cell' 200
    if ($before.data.state -ne $after.data.state -or $before.data.running -ne $after.data.running) {
        throw 'Cell state changed during preset CRUD'
    }
    Write-Output 'PASS preset CRUD, validation, start selector rejection; no RF start / 预设冒烟测试通过，未启动射频'
} finally {
    if ($created) { $null = Request DELETE "/presets/$id" 200 }
}
