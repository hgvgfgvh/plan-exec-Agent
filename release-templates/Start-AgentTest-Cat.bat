@echo off
chcp 65001 >nul
cd /d "%~dp0"
set "AGENTTEST_ROOT=%~dp0"
set "AGENTTEST_ROOT=%AGENTTEST_ROOT:~0,-1%"
echo Starting AgentTest Cat...
echo Install root: %AGENTTEST_ROOT%
start "" "%~dp0AgentTestCat.exe"
