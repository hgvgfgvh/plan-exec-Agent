# AgentTest

> **本地运行的多 Agent 内核（Go / Windows）**：以 `PlanAgent → BehaviorAgent` 为主链，支持 MCP（stdio/http）、外挂 Skill Pack、WebUI（HTTP+SSE）、以及旁路 RunView（回合日志 → HTML）。

![status](https://img.shields.io/badge/status-experimental-orange) ![language](https://img.shields.io/badge/Go-1.25-00ADD8) ![platform](https://img.shields.io/badge/platform-Windows-blue)

> ⚠️ **实验性项目**：能跑通“单次复杂任务”的编排与工具执行，但默认 WebUI 密码是常量、工具可操作本机（PowerShell/文件/邮件/浏览器自动化），请不要在不可信环境直接运行。

---

## ✨ 当前已实现（以代码为准）

- **主链编排**：用户输入 → `PlanAgent` 生成 TodoList → 逐步下发 `BehaviorAgent` 执行 → 汇总交付
- **Exec-Simple 快路径（可选）**：Plan 结合 MemoryHook 的经验命中与置信度，尝试走“单 episode 压缩执行”，失败自动降级为逐步 Exec
- **MCP 集成**：`capabilities.mcp` 支持 stdio/HTTP，工具对外公开名为 ``{server}__{tool}``
- **能力目录（运行时注入）**：执行类 Agent 的 system prompt 会注入第一层能力目录；二层 Schema 用 `get_capability_details`
- **外挂 Skill Pack**：扫描 `WorkSpace/skill_packs/*/SKILL.md`，可热更新目录（pack 内 `mcp.yaml` 仍需重启）
- **WebUI**：HTTP + SSE，支持登录、聊天、上传附件（落盘 `WorkSpace/inbox/{turn_id}/`）、RunView 查看
- **RunView（旁路）**：监听 `WorkSpace/logs/turns/*.json`，生成 `WorkSpace/run_views/*.html`

---

## 🚀 快速开始

### 环境要求

- Windows 10/11
- Go 1.25+
- （可选）Python 3.x：仅在启用 `python-sandbox` MCP 时需要本机 `python` 可执行

> 说明：示例配置默认走 `WorkSpace/mcp_bundled/` 的 bundled MCP。Node MCP 所需的 Node 运行时已随包放在 `WorkSpace/mcp_bundled/_runtime/`，一般不需要你另外装 Node。

### 配置

复制示例配置并填写你自己的 Key（**不要提交到 GitHub**）：

```powershell
cp config/app.example.yaml config/app.yaml
```

最少需要关注这些字段：

```yaml
agents:
  default_model: "deepSeek-onnx"

integrations:
  dashscope:
    api_key: "sk-..."        # 通义：联网搜索 / TTS / 视觉 / Embedding
  deepseek_legacy:
    api_key: "sk-..."        # DeepSeek：主链对话 / 计划编排

web:
  enabled: true
  listen: ":8765"
```

> 完整字段说明（逐项注释）见 `config/app.yaml` 与 `config/app.example.yaml`。

### 启动

```powershell
go run .
# 或
go build -o AgentTest.exe .
.\AgentTest.exe
```

指定其它配置文件：

```powershell
$env:AGENTTEST_CONFIG="config/app.custom.yaml"
.\AgentTest.exe
```

### 使用 WebUI

浏览器打开 `http://localhost:8765`：

- **用户名**：`admin`
- **默认密码**：`ZAQ!2wsx`（写死常量，见 `webui/server.go`）

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

由 `agent/agentManager.go` 初始化：

- `planAgent`（主入口，编排）
- `behaviorAgent`（逐步执行）
- `execSimpleAgent`（可选：存在才允许走 Exec-Simple）
- `interactiveAgent`（对话脑区；目前主要用于 Router 旧链路/反思链实验，不是默认入口）
- `routerAgent`（丘脑路由；已实现 dispatcher 快路径与 LLM 兜底，但当前 portal 主链默认不经过它）
- `baseAgent`（通用对话/参考实现）

---

## 🔌 MCP 与能力目录

- **公开工具名**：`{server}__{tool}`，例如 `sqlite__read_query`
- **渐进披露**：执行类 Agent 的 system 里会注入第一层目录；要看 Schema/全文用 `get_capability_details`
- **是否启用**：`config/app.yaml` 的 `capabilities.mcp.enabled`

---

## 🖥 RunView（回合运行视图）

- **输入**：`WorkSpace/logs/turns/*.json`（由 `turnjournal` 写入）
- **输出**：`WorkSpace/run_views/*.html`
- **开关**：`config.app.yaml` 的 `run_view.enabled`

示例独立配置见 `config/run_view.example.yaml`（RunView 的 LLM 配置与主链模型解耦）。

---

## 🛡 安全与开源发布建议（务必看）

- **不要把 WebUI 暴露到公网**：默认密码是常量，没有重置流程
- **不要提交敏感信息**：
  - `config/app.yaml`（你的真实 key）
  - `WorkSpace/`（日志、缓存、回合产物、附件）
  - `.env*`、IDE 配置（`.idea/` 等）
- 仓库默认 `.gitignore` 已忽略 `WorkSpace/*` 与 `.env*`；对外开源只提交 `config/app.example.yaml`

---

## 📄 License

待定 / TBD（公开前建议补一个明确 License，例如 MIT / Apache-2.0）。

