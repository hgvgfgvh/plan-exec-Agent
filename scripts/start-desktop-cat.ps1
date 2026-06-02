# 构建 AgentTest 内核 + 桌宠小猫，并启动小猫（含首次 API Key 向导）
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Set-Location $Root
Write-Host "==> go build AgentTest.exe" -ForegroundColor Cyan
go build -o AgentTest.exe .

Write-Host "==> dotnet build AgentTestCat" -ForegroundColor Cyan
dotnet build (Join-Path $Root "desktop-cat\AgentTestCat.sln") -c Release

$CatExe = Join-Path $Root "desktop-cat\AgentTestCat\bin\Release\net8.0-windows\AgentTestCat.exe"
if (-not (Test-Path $CatExe)) { throw "未找到 $CatExe" }

$env:AGENTTEST_ROOT = $Root
Write-Host "==> 启动小猫 (AGENTTEST_ROOT=$Root)" -ForegroundColor Green
Start-Process -FilePath $CatExe -WorkingDirectory $Root
