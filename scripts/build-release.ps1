# 构建 Windows x64 发布包：解压即用，无需 Go / .NET SDK
# 输出：release/AgentTest-<version>-win-x64/
param(
    [string]$Version = "",
    [switch]$Zip,
    [switch]$SkipMcpCopy
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Utf8Bom = New-Object System.Text.UTF8Encoding $true
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

# --- 发布小猫（自包含目录发布，需本机 WebView2 运行时）---
Write-Host "==> dotnet publish AgentTestCat (win-x64 self-contained)" -ForegroundColor Cyan
$CatProj = Join-Path $Root "desktop-cat\AgentTestCat\AgentTestCat.csproj"
$CatOut = Join-Path $Stage "AgentTestCat"
if (Test-Path $CatOut) { Remove-Item $CatOut -Recurse -Force }
dotnet publish $CatProj -c Release -r win-x64 --self-contained -p:PublishSingleFile=false "-o:$CatOut"
if ($LASTEXITCODE -ne 0) { throw "dotnet publish failed: $LASTEXITCODE" }

$PublishedCat = Join-Path $CatOut "AgentTestCat.exe"
if (-not (Test-Path $PublishedCat)) { throw "publish failed: $PublishedCat" }

# --- 配置与静态资源 ---
$Dirs = @(
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

# 配置目录：仅 YAML 模板（不含 app.yaml 与 Go 源码）；首次运行由小猫生成 app.yaml
$CfgStage = Join-Path $Stage "config"
New-Item -ItemType Directory -Path $CfgStage -Force | Out-Null
foreach ($cfgName in @("app.example.yaml", "run_view.example.yaml")) {
    $cfgSrc = Join-Path $Root "config\$cfgName"
    if (Test-Path $cfgSrc) {
        Copy-Item $cfgSrc (Join-Path $CfgStage $cfgName) -Force
    }
}
$CfgReadme = @"
AgentTest 配置目录
==================
- app.example.yaml   配置模板（含字段说明）
- app.yaml           首次运行小猫向导后自动生成（含 API Key，请勿分享）

若尚未生成 app.yaml：
  双击 Start-AgentTest-Cat.bat，按向导填写 DeepSeek API Key 并保存。

高级用户也可手动：
  copy app.example.yaml app.yaml
  再用记事本编辑 app.yaml 填入密钥。
"@
[System.IO.File]::WriteAllText((Join-Path $CfgStage "README.txt"), $CfgReadme, $Utf8Bom)

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

$McpDst = Join-Path $ws "mcp_bundled"
if ($SkipMcpCopy -and (Test-Path $McpDst)) {
    Write-Host "==> skip mcp_bundled (already present)" -ForegroundColor DarkGray
} else {
    Write-Host "==> robocopy mcp_bundled (~300MB)" -ForegroundColor Yellow
    $McpSrc = Join-Path $Root "WorkSpace\mcp_bundled"
    if (-not (Test-Path $McpSrc)) { throw "missing source: $McpSrc" }
    # robocopy 成功时常返回 1；PowerShell Stop 会误判为失败并跳过后续步骤
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    robocopy $McpSrc $McpDst /E /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
    $robocopyExit = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($robocopyExit -ge 8) { throw "robocopy mcp_bundled failed: $robocopyExit" }
    if (-not (Test-Path (Join-Path $McpDst "mcp-memory\memory-mcp.exe"))) {
        throw "mcp_bundled incomplete after robocopy (exit=$robocopyExit)"
    }
}

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
$expDb = Join-Path (Join-Path $Root "experience") "experience.db"
if ($expDb -and (Test-Path -LiteralPath $expDb)) {
    Copy-Item -LiteralPath $expDb -Destination (Join-Path $Stage "experience\experience.db") -Force
}

# --- 文档与启动入口 ---
Copy-Item (Join-Path $Root "LICENSE") $Stage -Force -ErrorAction SilentlyContinue
Copy-Item (Join-Path $Root "README.md") (Join-Path $Stage "README.md") -Force
Copy-Item (Join-Path $Root "desktop-cat\README.md") (Join-Path $Stage "desktop-cat-README.md") -Force

[System.IO.File]::WriteAllText((Join-Path $Stage "VERSION.txt"), $Version + "`r`n", $Utf8Bom)
$PortableNote = @"
AgentTest portable install
==========================
This folder is self-contained. Keep all files together.
Config: config\app.yaml (created on first run from app.example.yaml)
Runtime data: WorkSpace\, memory\, experience\
Do not depend on the source repository path.
"@
[System.IO.File]::WriteAllText((Join-Path $Stage "PORTABLE.txt"), $PortableNote, $Utf8Bom)

# Launcher + readme: ASCII filenames only (see release-templates/)
$TplDir = Join-Path $Root "release-templates"
if (-not (Test-Path $TplDir)) { throw "Missing folder: $TplDir" }
Copy-Item -LiteralPath (Join-Path $TplDir "Start-AgentTest-Cat.bat") -Destination (Join-Path $Stage "Start-AgentTest-Cat.bat") -Force
Copy-Item -LiteralPath (Join-Path $TplDir "README-RELEASE.txt") -Destination (Join-Path $Stage "README-RELEASE.txt") -Force
Get-ChildItem -LiteralPath $Stage -File | Where-Object { $_.Name -match '[^\x00-\x7F]' } | Remove-Item -Force -ErrorAction SilentlyContinue

if ($Zip) {
    $ReleaseDir = Join-Path $Root "release"
    $ZipPath = Join-Path $ReleaseDir "$OutName.zip"
    if (Test-Path $ZipPath) { Remove-Item $ZipPath -Force }
    Write-Host "==> zip -> $ZipPath" -ForegroundColor Cyan
    if (Get-Command tar.exe -ErrorAction SilentlyContinue) {
        $tarArgs = @("-a", "-c", "-f", $ZipPath, "-C", $ReleaseDir, $OutName)
        & tar.exe @tarArgs
        if ($LASTEXITCODE -ne 0) { throw "tar.exe failed: $LASTEXITCODE" }
    } else {
        Write-Host "tar.exe not found, fallback Compress-Archive" -ForegroundColor Yellow
        Compress-Archive -Path $Stage -DestinationPath $ZipPath -CompressionLevel Optimal
    }
}

$sizeMb = [math]::Round((Get-ChildItem $Stage -Recurse -File | Measure-Object Length -Sum).Sum / 1MB, 1)
Write-Host ""
Write-Host "完成: $Stage" -ForegroundColor Green
Write-Host "体积约 ${sizeMb} MB"
if ($Zip) { Write-Host "ZIP: $(Join-Path $Root "release\$OutName.zip")" -ForegroundColor Green }
