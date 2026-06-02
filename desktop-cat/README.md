# AgentTest 桌面小猫（集成版）

WPF 桌宠客户端，已并入 AgentTest 仓库。启动流程：

1. **首次配置**：填写 DeepSeek API Key（必填）、阿里云 DashScope Key（选填，多模态扩展）
2. **自动写入** `config/app.yaml` 并启动 `AgentTest.exe`
3. **桌宠界面**：连接本机 WebUI（`http://127.0.0.1:8765`）后即可对话
4. **浏览器 WebUI**：内核就绪后会自动打开默认浏览器并完成登录（可在小猫「设置」里关闭「启动时打开浏览器并登录 WebUI」）

## 一键构建并启动

在仓库根目录：

```powershell
.\scripts\start-desktop-cat.ps1
```

或手动：

```powershell
go build -o AgentTest.exe .
dotnet build desktop-cat\AgentTestCat.sln -c Release
$env:AGENTTEST_ROOT = (Get-Location).Path
.\desktop-cat\AgentTestCat\bin\Release\net8.0-windows\AgentTestCat.exe
```

> 小猫会从 exe 所在目录向上查找含 `config/app.yaml` 的工程根；也可用环境变量 `AGENTTEST_ROOT` 指定。

## 配置说明

| 向导字段 | 写入位置 | 用途 |
|----------|----------|------|
| DeepSeek API Key | `integrations.deepseek_legacy.api_key` 及 Memory/Soul/RunView 等 | 主链对话、计划编排 |
| 阿里云 API Key | `integrations.dashscope.api_key` | OCR、视觉、联网搜索、TTS |
| WebUI 密码（选填） | `web.password` | 小猫登录 WebUI |

小猫本机偏好仍保存在 `%AppData%\AgentTestPCAPPCat\settings.json`。

托盘菜单 **「API Key 配置…」** 可随时修改 Key（修改后建议重启小猫与内核）。

## 发布目录建议

将以下文件放在同一发行包中：

```
AgentTest.exe
config/app.yaml
WorkSpace/mcp_bundled/   （及运行时依赖）
AgentTestCat.exe
```

或将 `AgentTestCat.exe` 放在 `desktop-cat/AgentTestCat/bin/Release/net8.0-windows/`，从仓库根通过 `start-desktop-cat.ps1` 启动。
