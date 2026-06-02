@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo Starting AgentTest Cat...
start "" "%~dp0AgentTestCat.exe"
