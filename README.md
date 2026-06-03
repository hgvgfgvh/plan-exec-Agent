# AgentTest

> **本地运行的多 Agent 内核（Go / Windows）**：以 `PlanAgent → BehaviorAgent` 为主链，支持 MCP（stdio/http）、外挂 Skill Pack、WebUI（HTTP+SSE）、以及旁路 RunView（回合日志 → HTML）。

![status](https://img.shields.io/badge/status-experimental-orange) ![language](https://img.shields.io/badge/Go-1.25-00ADD8) ![platform](https://img.shields.io/badge/platform-Windows-blue)

> ⚠️ **实验性项目**：工具可操控本机（PowerShell / 文件 / 邮件 / 浏览器自动化）。请勿将 WebUI 暴露到公网；**不要**把含真实 Key 的 `config/app.yaml` 提交到 Git。

---

## 🐱 小猫一键体验（推荐 · 产品化入口）

**不想手改 YAML？** 用集成在仓库里的 **桌面小猫（WPF）**：首次配置 API Key → 自动启动内核 → 自动打开浏览器 WebUI → 桌宠对话。

### 你需要

| 项目 | 说明 |
|------|------|
| 系统 | Windows 10/11 |
| 运行时 | [.NET 8 桌面运行时](https://dotnet.microsoft.com/download/dotnet/8.0)（运行小猫） |
| 构建 | [Go 1.25+](https://go.dev/dl/) + [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)（仅首次编译） |
| 密钥 | [DeepSeek](https://platform.deepseek.com/) API Key（必填）；[阿里云 DashScope](https://dashscope.aliyun.com/) Key（选填，OCR / 视觉 / 联网搜索 / TTS） |

### 方式 A：下载 Release（免编译，推荐）

在 [GitHub Releases](https://github.com/hgvgfgvh/plan-exec-Agent/releases) 下载 `AgentTest-*-win-x64.zip`（或本地构建见下），解压后：

1. 双击 **Start-AgentTest-Cat.bat**（解压包内英文文件名，避免乱码）
2. 首次填写 API Key → 自动启动内核 + 浏览器 + 桌宠

详见解压包内 **使用说明.txt**。

本地打 **解压即用** 发布包（配置与 `WorkSpace` 均在包内，不依赖源码目录）：

```powershell
.\scripts\build-release.ps1 -Zip
# 产出：release\AgentTest-<日期>-win-x64\ 与同名 .zip（约 190–400MB，含 mcp_bundled）
# 用户解压后双击 Start-AgentTest-Cat.bat，首次向导写入 config\app.yaml
```

### 方式 B：克隆源码自行编译

```powershell
git clone https://github.com/hgvgfgvh/plan-exec-Agent.git
cd plan-exec-Agent

copy config\app.example.yaml config\app.yaml
.\scripts\start-desktop-cat.ps1
```

**首次启动**会弹出配置向导：

1. **DeepSeek API Key**（必填）— 主链对话、计划编排  
2. **阿里云 DashScope Key**（选填）— 多模态扩展  
3. **WebUI 密码**（选填）— 不填则使用 `app.example.yaml` 中的 `change-me`  

保存后自动：写入本机 `config/app.yaml` → 启动 `AgentTest.exe` → 打开浏览器登录 WebUI → 显示桌宠。

### 日常使用

| 方式 | 操作 |
|------|------|
| 推荐 | 运行 `.\scripts\start-desktop-cat.ps1` |
| 直接运行 | 在仓库根目录双击 / 运行 `desktop-cat\AgentTestCat\bin\Release\net8.0-windows\AgentTestCat.exe` |
| 仅内核 | `go build -o AgentTest.exe .` 后 `.\AgentTest.exe`，浏览器访问 `http://127.0.0.1:8765` |

托盘菜单：**显示小猫** · **打开 WebUI** · **API Key 配置** · **连接设置** · **退出**

小猫偏好保存在 `%AppData%\AgentTestPCAPPCat\settings.json`（与 Git 无关）。

更多细节见 [desktop-cat/README.md](desktop-cat/README.md)。

---

## ✨ 当前已实现（以代码为准）

- **主链编排**：用户输入 → `PlanAgent` 生成 TodoList → 逐步下发 `BehaviorAgent` 执行 → 汇总交付
- **Exec-Simple 快路径（可选）**：Plan 结合 MemoryHook 的经验命中与置信度，尝试走“单 episode 压缩执行”，失败自动降级为逐步 Exec
- **MCP 集成**：`capabilities.mcp` 支持 stdio/HTTP，工具对外公开名为 ``{server}__{tool}``
- **能力目录（运行时注入）**：执行类 Agent 的 system prompt 会注入第一层能力目录；二层 Schema 用 `get_capability_details`
- **外挂 Skill Pack**：扫描 `WorkSpace/skill_packs/*/SKILL.md`，可热更新目录（pack 内 `mcp.yaml` 仍需重启）
- **WebUI**：HTTP + SSE，支持登录、聊天、上传附件（落盘 `WorkSpace/inbox/{turn_id}/`）、RunView 查看
- **RunView（旁路）**：监听 `WorkSpace/logs/turns/*.json`，生成 `WorkSpace/run_views/*.html`
- **桌面小猫**：见上文 **🐱 小猫一键体验**

---

## 🚀 开发者快速开始（手改配置）

### 环境要求

- Windows 10/11
- Go 1.25+
- （可选）Python 3.x：仅在启用 `python-sandbox` MCP 时需要本机 `python` 可执行

> bundled MCP 在 `WorkSpace/mcp_bundled/`（被 `.gitignore` 忽略，需自行准备或从发布包拷贝）。Node 类 MCP 可使用包内 `WorkSpace/mcp_bundled/_runtime/` 的 Node。

### 配置

```powershell
copy config\app.example.yaml config\app.yaml
```

最少填写：

```yaml
integrations:
  deepseek_legacy:
    api_key: "your-deepseek-api-key"
  dashscope:
    api_key: "your-dashscope-api-key"   # 可选

web:
  enabled: true
  listen: ":8765"
  username: "admin"
  password: "change-me"
```

完整字段见 `config/app.example.yaml`。

### 启动内核

```powershell
go build -o AgentTest.exe .
.\AgentTest.exe
```

指定配置：`$env:AGENTTEST_CONFIG="config/app.custom.yaml"`

### WebUI 登录

- 地址：`http://127.0.0.1:8765`
- 用户：`admin`（可在 `web.username` 修改）
- 密码：`config/app.yaml` 的 `web.password`（示例默认为 `change-me`；代码默认兜底见 `config/config.go`）

---

## 🧠 系统构成（简版）

### 关键模块

- **`interaction.Router`**：统一入站标注、回合上下文注入（`turn_id`）、以及回执绑定
- **`portal.ProcessTurn`**：把一条入站消息送入主链（Plan 优先；无 Plan 时回退直通 Behavior）
- **`PlanAgent`**：只负责“拆分/调节/归档/汇总”（自身不直接调用 MCP 工具）
- **`BehaviorAgent`**：执行器（工具循环 + 内置技能树 + MCP 工具 + 外挂 SKILL）
- **`capabilities`**：MCP 连接与工具注册、能力目录、审计/可观测配置
- **`runview`**：旁路监听回合日志，生成 HTML

### Agent 列表（当前实际注册）

由 `agent/agentManager.go` 初始化：`planAgent`、`behaviorAgent`、`execSimpleAgent`、`interactiveAgent`、`routerAgent`、`baseAgent`。

---

## 🔌 MCP 与能力目录

- **公开工具名**：`{server}__{tool}`，例如 `sqlite__read_query`
- **渐进披露**：第一层目录在 system 中；Schema 用 `get_capability_details`
- **启用**：`config/app.yaml` → `capabilities.mcp.enabled`；**修改 MCP 列表后需重启内核**

---

## 🖥 RunView（回合运行视图）

- **输入**：`WorkSpace/logs/turns/*.json`
- **输出**：`WorkSpace/run_views/*.html`
- **开关**：`run_view.enabled`（LLM 配置与主链解耦，见 `config/run_view.example.yaml`）

---

## 🛡 安全与开源（提交前自查）

| 可以提交 | 不要提交 |
|----------|----------|
| `config/app.example.yaml` | `config/app.yaml`（真实 Key / 密码） |
| 源码、`desktop-cat/`、`scripts/` | `WorkSpace/*`（日志、产物、mcp 数据） |
| | `.env*`、`.idea/`、`*.exe` |

仓库 `.gitignore` 已忽略 `config/app.yaml` 与 `WorkSpace/*`。克隆后请 `copy config\app.example.yaml config\app.yaml` 或由小猫向导生成。

---

## 📄 License

MIT（见仓库根目录 `LICENSE`）。
