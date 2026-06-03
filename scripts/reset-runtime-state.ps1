# Reset copy\AgentTest runtime data (no chat history, soul, memory MCP, logs, todos)
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

function Set-EmptyFile([string]$Path) {
    $dir = Split-Path $Path -Parent
    if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    [System.IO.File]::WriteAllText($Path, "", (New-Object System.Text.UTF8Encoding $false))
}

Write-Host "==> Reset: $Root" -ForegroundColor Cyan

Set-EmptyFile (Join-Path $Root "memory\my_agent_memory.jsonl")
Set-EmptyFile (Join-Path $Root "memory\plan_agent_memory.jsonl")

$expDb = Join-Path $Root "experience\experience.db"
if (Test-Path -LiteralPath $expDb) { Remove-Item -LiteralPath $expDb -Force }
Set-EmptyFile (Join-Path $Root "experience\experience.jsonl")

$ws = Join-Path $Root "WorkSpace"
$keep = @("mcp_bundled", ".gitkeep")
Get-ChildItem -LiteralPath $ws -Force | Where-Object { $_.Name -notin $keep } | ForEach-Object {
    Remove-Item -LiteralPath $_.FullName -Recurse -Force
}

$subdirs = "logs\turns", "logs\llm_chat", "run_views", "mcp_data\memory", "mcp_data\soul", "mcp_data\mcp-manager", "ToDoList", "inbox", "skill_packs", "word", "ppt", "databases", "playwright_out"
foreach ($d in $subdirs) {
    New-Item -ItemType Directory -Path (Join-Path $ws $d) -Force | Out-Null
}

$dbSrc = Join-Path $ws "databases\savegame_template.db"
$altDb = "C:\DATA\GODATA\AgentTest\WorkSpace\databases\savegame_template.db"
if (-not (Test-Path -LiteralPath $dbSrc) -and (Test-Path -LiteralPath $altDb)) {
    Copy-Item -LiteralPath $altDb -Destination $dbSrc -Force
}

$catDir = Join-Path $env:APPDATA "AgentTestPCAPPCat"
$dbg = Join-Path $catDir "debug.log"
if (Test-Path -LiteralPath $dbg) { Remove-Item -LiteralPath $dbg -Force }

Write-Host "==> Done. Restart AgentTest / Cat from repo root." -ForegroundColor Green
