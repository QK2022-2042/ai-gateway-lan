<#
.SYNOPSIS
  卸载 AI Gateway Windows 服务（计划任务）。

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\uninstall-windows.ps1

.EXAMPLE
  # 同时删除配置、日志、数据库和 master.key（不可恢复）
  powershell -ExecutionPolicy Bypass -File .\uninstall-windows.ps1 -RemoveData
#>
[CmdletBinding()]
param(
    [string]$TaskName = "AI Gateway",
    [string]$InstallDir = "C:\ai-gateway",
    [switch]$RemoveData
)

$ErrorActionPreference = "Stop"

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "错误：请以管理员身份运行 PowerShell。" -ForegroundColor Red
    exit 1
}

Write-Host "==> 停止并删除计划任务 $TaskName" -ForegroundColor Cyan
Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Get-Process -Name "ai-gateway" -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "==> 删除防火墙规则" -ForegroundColor Cyan
Remove-NetFirewallRule -DisplayName "AI Gateway" -ErrorAction SilentlyContinue

$exe = Join-Path $InstallDir "ai-gateway.exe"
if (Test-Path -LiteralPath $exe) {
    Remove-Item -LiteralPath $exe -Force -ErrorAction SilentlyContinue
}

if ($RemoveData) {
    $configDir = "C:\ProgramData\ai-gateway"
    Write-Warning "即将删除 $configDir（包含 gateway.db、master.key、配置和日志），此操作不可恢复。"
    Remove-Item -LiteralPath $configDir -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $configDir) {
        Write-Warning "部分文件删除失败，请手动删除 $configDir"
    }
} else {
    Write-Host "已保留数据：C:\ProgramData\ai-gateway"
    Write-Host "需要彻底删除时重新运行并加 -RemoveData。"
}

Write-Host ""
Write-Host "卸载完成。安装目录 $InstallDir 中的脚本和 README 未自动删除。" -ForegroundColor Green
Write-Host "确认不再需要后，可手动删除该目录。"
