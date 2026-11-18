<#
.SYNOPSIS
  AI Gateway Windows 10 安装脚本（无需 Docker / Python / Node）

.DESCRIPTION
  安装 ai-gateway.exe，创建数据目录和配置，注册开机自启计划任务，
  放行防火墙端口，并启动服务。重复执行即为升级，会保留数据库、主密钥与配置。

.EXAMPLE
  以管理员身份打开 PowerShell，进入解压目录后执行：
  powershell -ExecutionPolicy Bypass -File .\install-windows.ps1

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\install-windows.ps1 -Port 9000 -SiteName "公司模型网关"
#>
[CmdletBinding()]
param(
    [string]$InstallDir = "C:\ai-gateway",
    [string]$DataDir = "C:\ProgramData\ai-gateway\data",
    [int]$Port = 8080,
    [string]$SiteName = "AI 模型中转站",
    [string]$TaskName = "AI Gateway",
    [switch]$NoFirewall
)

$ErrorActionPreference = "Stop"

function Fail([string]$Message) {
    Write-Host ""
    Write-Host "错误：$Message" -ForegroundColor Red
    exit 1
}

function Info([string]$Message) {
    Write-Host "==> $Message" -ForegroundColor Cyan
}

# ---- 前置检查 ----
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Fail "请以管理员身份运行。右键开始菜单中的 PowerShell，选择“以管理员身份运行”，再执行本脚本。"
}

if ($Port -lt 1 -or $Port -gt 65535) {
    Fail "端口无效：$Port"
}

$SourceExe = Join-Path $PSScriptRoot "ai-gateway.exe"
if (-not (Test-Path -LiteralPath $SourceExe)) {
    Fail "当前目录缺少 ai-gateway.exe。请先把 zip 完整解压到一个目录，再在该目录中运行本脚本。"
}

$ConfigDir = "C:\ProgramData\ai-gateway"
$ConfigPath = Join-Path $ConfigDir "config.json"
$LogDir = Join-Path $ConfigDir "logs"
$LogPath = Join-Path $LogDir "gateway.log"
$ExePath = Join-Path $InstallDir "ai-gateway.exe"

Info "创建目录"
New-Item -ItemType Directory -Force -Path $InstallDir, $DataDir, $ConfigDir, $LogDir | Out-Null

# ---- 停止旧任务，便于覆盖二进制 ----
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Info "停止旧服务"
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}
Get-Process -Name "ai-gateway" -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

# ---- 安装二进制 ----
Info "安装程序到 $ExePath"
$tempExe = "$ExePath.new"
Copy-Item -LiteralPath $SourceExe -Destination $tempExe -Force
Move-Item -LiteralPath $tempExe -Destination $ExePath -Force

# ---- 配置文件（已存在则保留） ----
if (Test-Path -LiteralPath $ConfigPath) {
    Info "保留已有配置 $ConfigPath"
    try {
        $config = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
    } catch {
        Fail "已有配置文件不是合法 JSON：$ConfigPath"
    }
    if ($config.data_dir) { $DataDir = [string]$config.data_dir }
    if ($config.log_file) {
        $LogPath = [string]$config.log_file
        $LogDir = Split-Path -Parent $LogPath
    } else {
        # 兼容早期没有 log_file 的配置，避免计划任务运行时日志无处可查。
        $config | Add-Member -NotePropertyName log_file -NotePropertyValue $LogPath -Force
        $json = $config | ConvertTo-Json
        [System.IO.File]::WriteAllText(
            $ConfigPath,
            $json,
            (New-Object System.Text.UTF8Encoding($false))
        )
        Info "已为旧配置补充日志路径：$LogPath"
    }
    if ($config.listen -match ':(\d+)$') { $Port = [int]$Matches[1] }
} else {
    Info "生成配置 $ConfigPath"
    $config = [ordered]@{
        listen            = ":$Port"
        data_dir          = $DataDir
        site_name         = $SiteName
        trust_proxy       = $false
        session_ttl_hours = 168
        log_file          = $LogPath
    }
    $json = $config | ConvertTo-Json
    # Windows PowerShell 5.1 的 Set-Content -Encoding UTF8 会写 BOM，Go 的
    # JSON 解析器不接受 BOM，因此显式写无 BOM 的 UTF-8。
    [System.IO.File]::WriteAllText(
        $ConfigPath,
        $json,
        (New-Object System.Text.UTF8Encoding($false))
    )
}

New-Item -ItemType Directory -Force -Path $DataDir, $LogDir | Out-Null

# ---- 限制数据目录权限：仅 SYSTEM 与管理员可读 ----
Info "收紧数据目录权限"
& icacls.exe $DataDir /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Warning "设置数据目录 ACL 失败，请手动检查 $DataDir 权限。" }
& icacls.exe $LogDir /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Warning "设置日志目录 ACL 失败，请手动检查 $LogDir 权限。" }
& icacls.exe $ConfigPath /inheritance:r /grant:r "*S-1-5-18:F" "*S-1-5-32-544:F" | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Warning "设置配置文件 ACL 失败，请手动检查 $ConfigPath 权限。" }

# ---- 计划任务：开机以 SYSTEM 身份后台运行 ----
Info "注册开机自启计划任务"
$action = New-ScheduledTaskAction -Execute $ExePath `
    -Argument "-config `"$ConfigPath`"" -WorkingDirectory $InstallDir
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries -StartWhenAvailable `
    -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit ([TimeSpan]::Zero)
$taskPrincipal = New-ScheduledTaskPrincipal -UserId "SYSTEM" `
    -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Settings $settings -Principal $taskPrincipal -Force `
    -Description "AI Gateway - OpenAI-compatible model API relay" | Out-Null

# ---- 防火墙 ----
if (-not $NoFirewall) {
    Info "配置防火墙规则（TCP $Port）"
    try {
        Remove-NetFirewallRule -DisplayName "AI Gateway" -ErrorAction SilentlyContinue
        New-NetFirewallRule -DisplayName "AI Gateway" -Direction Inbound `
            -Action Allow -Protocol TCP -LocalPort $Port -Profile Any | Out-Null
    } catch {
        Write-Warning "防火墙规则创建失败，请手动放行 TCP $Port。错误：$_"
    }
}

# ---- 启动并等待健康检查 ----
Info "启动服务"
Start-ScheduledTask -TaskName $TaskName

$ready = $false
for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 500
    try {
        $resp = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/healthz" -TimeoutSec 2
        if ($resp.StatusCode -eq 200) { $ready = $true; break }
    } catch {
        # 服务尚未监听，继续等待
    }
}

if (-not $ready) {
    Write-Host ""
    Write-Warning "服务未在 20 秒内就绪。"
    Write-Host "计划任务状态："
    Get-ScheduledTaskInfo -TaskName $TaskName -ErrorAction SilentlyContinue | Format-List
    if (Test-Path -LiteralPath $LogPath) {
        Write-Host "最近日志（$LogPath）："
        Get-Content -LiteralPath $LogPath -Tail 30
    } else {
        Write-Host "日志文件尚未生成：$LogPath"
    }
    Write-Host "常见原因：端口被占用、配置路径错误、杀毒软件拦截。"
    exit 1
}

$ip = $null
try {
    $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object {
            $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*"
        } |
        Select-Object -First 1 -ExpandProperty IPAddress
} catch { }
if (-not $ip) { $ip = "localhost" }

Write-Host ""
Write-Host "============================================================"
Write-Host "  安装完成"
Write-Host "    管理界面 : http://${ip}:$Port"
Write-Host "    本机访问 : http://127.0.0.1:$Port"
Write-Host "    程序目录 : $InstallDir"
Write-Host "    配置目录 : $ConfigDir"
Write-Host "    数据目录 : $DataDir"
Write-Host "    日志文件 : $LogPath"
Write-Host ""
Write-Host "  开机自启任务：$TaskName"
Write-Host "    查看状态 : Get-ScheduledTaskInfo -TaskName `"$TaskName`""
Write-Host "    停止服务 : Stop-ScheduledTask -TaskName `"$TaskName`""
Write-Host "    启动服务 : Start-ScheduledTask -TaskName `"$TaskName`""
Write-Host "    实时日志 : Get-Content `"$LogPath`" -Tail 50 -Wait"
Write-Host ""
$adminFile = Join-Path $DataDir "initial-admin.txt"
if (Test-Path -LiteralPath $adminFile) {
    Write-Host "  首次登录凭据（登录后请立即改密并删除该文件）：" -ForegroundColor Yellow
    Get-Content -LiteralPath $adminFile | ForEach-Object { Write-Host "    $_" }
    Write-Host "    删除命令 : Remove-Item -LiteralPath `"$adminFile`""
} else {
    Write-Host "  数据目录中已有账号，请使用既有管理员密码登录。"
}
Write-Host ""
Write-Host "  如仍无法从其他电脑访问，请检查 Windows 防火墙和网络位置类型。"
Write-Host "============================================================"
