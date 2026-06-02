# 构建 Windows x64 发布包：解压即用，无需 Go / .NET SDK
# 输出：release/AgentTest-<version>-win-x64/
param(
    [string]$Version = "",
    [switch]$Zip
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Date -Format "yyyy.MM.dd")
}

$OutName = "AgentTest-$Version-win-x64"
$OutDir = Join-Path $Root "release\$OutName"
$Stage = $OutDir

Write-Host "==> Release $OutName" -ForegroundColor Cyan
if (Test-Path $Stage) { Remove-Item $Stage -Recurse -Force }
New-Item -ItemType Directory -Path $Stage -Force | Out-Null

# --- 编译内核 ---
Write-Host "==> go build AgentTest.exe" -ForegroundColor Cyan
go build -ldflags="-s -w" -o (Join-Path $Stage "AgentTest.exe") .

# --- 发布小猫（自包含单文件，需本机已装 WebView2 运行时）---
Write-Host "==> dotnet publish AgentTestCat (win-x64 self-contained)" -ForegroundColor Cyan
$CatProj = Join-Path $Root "desktop-cat\AgentTestCat\AgentTestCat.csproj"
dotnet publish $CatProj -c Release -r win-x64 --self-contained true `
    -p:PublishSingleFile=true `
    -p:IncludeNativeLibrariesForSelfExtract=true `
    -p:EnableCompressionInSingleFile=true `
    -o (Join-Path $Stage "_publish_cat")

$PublishedCat = Join-Path $Stage "_publish_cat\AgentTestCat.exe"
if (-not (Test-Path $PublishedCat)) { throw "publish failed: $PublishedCat" }
Move-Item $PublishedCat (Join-Path $Stage "AgentTestCat.exe") -Force
Remove-Item (Join-Path $Stage "_publish_cat") -Recurse -Force

# --- 配置与静态资源 ---
$Dirs = @(
    "config",
    "behavior",
    "agent\soul",
    "memory",
    "experience"
)
foreach ($d in $Dirs) {
    $src = Join-Path $Root $d
    if (Test-Path $src) {
        $dest = Join-Path $Stage $d
        New-Item -ItemType Directory -Path (Split-Path $dest -Parent) -Force -ErrorAction SilentlyContinue | Out-Null
        Copy-Item $src $dest -Recurse -Force
    }
}

# 仅保留默认人格 YAML（减小包体）
$SoulDir = Join-Path $Stage "agent\soul"
if (Test-Path $SoulDir) {
    Get-ChildItem $SoulDir -File | Where-Object { $_.Name -ne "Nexus.yml" } | Remove-Item -Force
}

# 配置：仅示例 + 首次运行由小猫生成 app.yaml
Copy-Item (Join-Path $Root "config\app.example.yaml") (Join-Path $Stage "config\app.example.yaml") -Force
if (Test-Path (Join-Path $Stage "config\app.yaml")) { Remove-Item (Join-Path $Stage "config\app.yaml") -Force }

# 空 memory / experience 占位
$memDir = Join-Path $Stage "memory"
New-Item -ItemType Directory -Path $memDir -Force | Out-Null
foreach ($f in @("my_agent_memory.jsonl", "plan_agent_memory.jsonl")) {
    $p = Join-Path $memDir $f
    if (-not (Test-Path $p)) { Set-Content -Path $p -Value "" -Encoding UTF8 }
}

# --- WorkSpace：bundled MCP + 空目录骨架（不拷贝用户日志/记忆数据）---
$ws = Join-Path $Stage "WorkSpace"
New-Item -ItemType Directory -Path $ws -Force | Out-Null

Write-Host "==> robocopy mcp_bundled (约 300MB，请稍候)" -ForegroundColor Yellow
$McpSrc = Join-Path $Root "WorkSpace\mcp_bundled"
$McpDst = Join-Path $ws "mcp_bundled"
robocopy $McpSrc $McpDst /E /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
if ($LASTEXITCODE -ge 8) { throw "robocopy mcp_bundled failed: $LASTEXITCODE" }

$wsSub = @(
    "skill_packs",
    "logs\turns",
    "logs\llm_chat",
    "run_views",
    "mcp_data\memory",
    "mcp_data\soul",
    "mcp_data\mcp-manager",
    "inbox",
    "word",
    "ppt",
    "databases"
)
foreach ($sub in $wsSub) {
    $p = Join-Path $ws $sub
    New-Item -ItemType Directory -Path $p -Force | Out-Null
    Set-Content -Path (Join-Path $p ".gitkeep") -Value "" -Encoding UTF8
}

# SQLite 模板库
$dbSrc = Join-Path (Join-Path $Root "WorkSpace") "databases\savegame_template.db"
if ($dbSrc -and (Test-Path -LiteralPath $dbSrc)) {
    Copy-Item -LiteralPath $dbSrc -Destination (Join-Path $ws "databases\savegame_template.db") -Force
}

# experience.db
$expDb = Join-Path $Root "experience\experience.db"
if (Test-Path $expDb) {
    Copy-Item $expDb (Join-Path $Stage "experience\experience.db") -Force
}

# --- 文档与启动入口 ---
Copy-Item (Join-Path $Root "LICENSE") $Stage -Force -ErrorAction SilentlyContinue
Copy-Item (Join-Path $Root "README.md") (Join-Path $Stage "README.md") -Force
Copy-Item (Join-Path $Root "desktop-cat\README.md") (Join-Path $Stage "desktop-cat-README.md") -Force

Set-Content -Path (Join-Path $Stage "VERSION.txt") -Value $Version -Encoding UTF8

@'
@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo 正在启动 AgentTest 小猫...
start "" "%~dp0AgentTestCat.exe"
'@ | Set-Content -Path (Join-Path $Stage "启动 AgentTest 小猫.bat") -Encoding UTF8

@'
# AgentTest Windows 发布包 — 使用说明

版本见 VERSION.txt

## 系统要求

- Windows 10/11 x64
- 可访问互联网的 API（DeepSeek / 可选阿里云 DashScope）
- [Microsoft Edge WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)（多数 Win11 已自带）

**无需**安装 Go、.NET SDK 或 Node.js。

## 快速开始

1. 解压本文件夹到任意路径（路径尽量不要含特殊字符）
2. 双击 **启动 AgentTest 小猫.bat**（或 **AgentTestCat.exe**）
3. 首次运行填写 **DeepSeek API Key**（必填）与可选 **DashScope Key**
4. 保存后自动启动内核、打开浏览器 WebUI、显示桌宠

## 目录说明

| 路径 | 说明 |
|------|------|
| AgentTest.exe | Go 内核（小猫会自动拉起，一般无需单独双击） |
| AgentTestCat.exe | 桌面小猫（推荐入口） |
| config/app.example.yaml | 配置模板；首次配置后生成 config/app.yaml |
| WorkSpace/mcp_bundled/ | 内置 MCP（filesystem / sqlite / 浏览器等） |
| WorkSpace/ | 运行时产物、日志、上传附件（可删日志减负） |

## 仅启动内核（高级）

```powershell
cd /d 本目录
copy config\app.example.yaml config\app.yaml
# 编辑 config\app.yaml 填入 API Key
AgentTest.exe
```

浏览器访问 http://127.0.0.1:8765

## 安全提示

- 勿将 WebUI 暴露到公网
- config/app.yaml 含密钥，勿分享
- 工具可执行本机 PowerShell / 文件 / 邮件 / 浏览器操作，请在可信环境使用

## 开源仓库

https://github.com/hgvgfgvh/plan-exec-Agent
'@ | Set-Content -Path (Join-Path $Stage "使用说明.txt") -Encoding UTF8

if ($Zip) {
    $ZipPath = Join-Path $Root "release\$OutName.zip"
    if (Test-Path $ZipPath) { Remove-Item $ZipPath -Force }
    Write-Host "==> Compress-Archive -> $ZipPath" -ForegroundColor Cyan
    Compress-Archive -Path $Stage -DestinationPath $ZipPath -CompressionLevel Optimal
}

$sizeMb = [math]::Round((Get-ChildItem $Stage -Recurse -File | Measure-Object Length -Sum).Sum / 1MB, 1)
Write-Host ""
Write-Host "完成: $Stage" -ForegroundColor Green
Write-Host "体积约 ${sizeMb} MB"
if ($Zip) { Write-Host "ZIP: $(Join-Path $Root "release\$OutName.zip")" -ForegroundColor Green }
