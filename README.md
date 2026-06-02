# AgentTest

> **本地运行的多 Agent 内核（脑分区架构）—— Go 实现，支持 MCP / 桌面自动化 / 长期记忆 / 人格切换。**

![status](https://img.shields.io/badge/status-experimental-orange) ![language](https://img.shields.io/badge/Go-1.25-00ADD8) ![platform](https://img.shields.io/badge/platform-Windows-blue) ![license](https://img.shields.io/badge/license-TBD-lightgrey)

一个把"脑分区"作为一等抽象的 Agent 框架：

- **Router（丘脑）** 做分流，**Affective（情感）** 做对话，**Behavior（行为）** 做执行；
- **Blackboard（神经总线）** 解耦各分区，所有消息携带 `TurnID + Hop` 元信息；
- **Prefrontal Cortex（前额叶）** 跑工具循环 + 历史归档；
- **Body** 层挂接 ears / eyes / mouth 等设备适配器；
- **Capabilities** 层统一接入 MCP / LangChain Plugins / 外部 Skill Pack。

> ⚠️ **状态说明**：这是个**人开发的极客项目**，不是生产级产品。  
> 已经能跑「单次复杂任务」（写文档、发邮件、桌面自动化、知识问答）；  
> 但「持续监听 / 主动行为 / 可穿戴设备协议」仍在路线图，未实现。

---

## ✨ 已实现的能力

| 能力 | 说明 |
|---|---|
| 🧠 **多 Agent 协作** | 5 个独立 Agent（Router/Affective/Behavior/Base/DigitalHumanControl），通过黑板 pub/sub 解耦 |
| 🧭 **混合路由** | 规则 Dispatcher（确定性，零 LLM 成本）+ LLM 兜底（长尾灵活性） |
| 🛡 **反思链跳数预算** | `TurnID + Hop` 全链路追踪，超出 `router_reflection_max_hops` 自动熔断 |
| 🔌 **MCP 集成** | stdio / HTTP 两种 transport；支持随包分发（`mcp_bundled`） |
| 📦 **Skill Pack** | 扫描 `SKILL.md` 自动发现；可附带 `mcp.yaml` 合并 MCP |
| 🧰 **桌面自动化** | 键盘 / 鼠标 / PowerShell / 截图 / OCR / Dense Caption / 视觉定位 / UI 元素扫描 |
| 📄 **文档生成** | Word 报告 / PPT 幻灯片 / 带附件邮件 |
| 🌐 **联网检索** | Qwen3-Max AI Search Skill |
| 💾 **长期记忆** | RAG（多次召回同一关键词后固化为"直觉"） + 经验库（SQLite + JSONL） |
| 🎭 **人格切换** | `agent/soul/*.yml` 加载不同灵魂（Nexus / Misato / Asuka …） |
| 🖥 **内置 Web UI** | `:8765` HTTP + SSE，浏览器即可交互 |

---

## 🗺️ 路线图（未实现）

这些方向已经写在原 README 的愿景里，**目前架构里只是占位、需要新模块**：

- 🛣 **持续感知循环** —— ears / eyes 实时融合为统一情境，目前还是事件驱动
- 🛣 **意图栈 / 后台 goal** —— "我在监听 X / 等待 Y" 的持久化数据结构
- 🛣 **调度器** —— "下午 3 点提醒我"这类时间触发
- 🛣 **可穿戴设备协议** —— websocket / 串口接入外设
- 🛣 **主动 tick 循环** —— Agent 不依赖用户输入也能行动

> 想做"会议监听 + 自动整理文档"那个旗舰场景，缺的不是某个 skill，是上面这几个抽象。

---

## 🧠 架构总览

### 脑分区映射

```
                           ┌─────────────────────┐
                           │   用户 / WebUI       │
                           └──────────┬──────────┘
                                      │
                              portal.RunRouterTurn
                                      │
                                      ▼
                           ┌─────────────────────┐
                           │   Router (丘脑)      │
                           │   - 规则 Dispatcher  │  ← 80% 输入零 LLM 成本
                           │   - LLM 兜底         │
                           └──────────┬──────────┘
                                      │
                  ┌───────────────────┼───────────────────┐
                  │                                       │
                  ▼                                       ▼
       ┌─────────────────────┐                 ┌─────────────────────┐
       │ Affective (情感)     │                 │ Behavior (行为)      │
       │  - Soul 人格         │                 │  - Skill 三层树      │
       │  - 对话 / 知识问答    │                 │  - SetExecutorStep   │
       │  - 反思 / 主动决策    │                 │  - MCP / Skill Pack  │
       └──────────┬──────────┘                 └──────────┬──────────┘
                  │                                       │
                  └───────────────┬───────────────────────┘
                                  ▼
                  ┌────────────────────────────────┐
                  │  Blackboard (神经总线)          │
                  │  TurnID + Hop 全链路            │
                  └────────────────────────────────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            ▼                     ▼                     ▼
       ┌─────────┐           ┌─────────┐           ┌─────────┐
       │  Ears   │           │  Eyes   │           │  Mouth  │
       │ mic/wav │           │ screen/ │           │  TTS    │
       │         │           │ camera  │           │         │
       └─────────┘           └─────────┘           └─────────┘
```

### 消息流向（典型快路径）

```
用户输入 ──► RouterAgent.Process
                  │
                  ▼
            dispatcher.Classify
                  │
        ┌─────────┴─────────┐
        │ HighConfidence    │ LowConfidence
        ▼                   ▼
   fastDispatch       Executor.Run (LLM)
        │                   │
        ▼                   ▼
   PublishMsg(           Action: AffectiveInteractiveAgentInput
     TopicBehaviorInput, Action: BehaviorAgentAgentInput
     query,
     TurnID, Hop=0)
        │
        ▼
   BehaviorAgent.handleFeedback
        │
        ▼
   Executor.Run → SetExecutorStep(tree=...)
        │
        ▼
   ExecuteSkillTree (异步) → exec.status / exec.result
        │
        ▼
   BehaviorAgentAgentOutput → TopicBehaviorOutput (carries Hop)
        │
        ▼
   Router.handleFeedback (checks Hop < max → reflect / drop)
        │
        ▼
   facadeInteraction.output → portal → 用户
```

### 关键不变量

这套架构有四个"不会让你裸奔"的设计点：

1. **跳数预算（Hop budget）**  
   所有跨 Agent 消息携带 `Hop` 字段，Router 每反思一跳 `+1`；超出 `executor.router_reflection_max_hops`（默认 2）即丢弃。**反思链不会无限放大**。

2. **路由确定性优先**  
   `agent/dispatcher` 是纯函数规则分类器。命中即出 `Decision`，**不调用 LLM**。模糊语句才回退到 Executor。**80% 的输入零推理成本**。

3. **指令语言带 Schema**  
   `SetExecutorStep` 接受 `tree` 字段（`SkillStep` 嵌套对象）。Skill 名错、嵌套结构破——**JSON 解析阶段就报错，附完整路径**，不会进入执行期。

4. **topic 集中常量**  
   全仓 topic 字符串只在 `body/blackboard/topics.go` 出现一次；改名、加 schema、加追踪都只动一处。

---

## 🚀 快速开始

### 环境要求

- Windows 10/11（桌面自动化 Skill 依赖 Windows API）
- Go 1.25+
- 可选：Node.js + npx（如要用 `@modelcontextprotocol/server-filesystem`）
- 可选：Python 3.11 + `mcp-server-sqlite`（如要挂 SQLite MCP）

### 配置

```powershell
cp config/app.example.yaml config/app.yaml
```

最少需要确认这些：

```yaml
agents:
  default_model: "deepSeek-onnx"     # 看 manager/modelManager.go 里支持的 driver

paths:
  soul: "agent/soul/Nexus.yml"       # 切换人格：Nexus / KatsuragiMisato / Asuka 等

web:
  enabled: true                      # 想用浏览器交互就开
  listen: ":8765"
```

模型驱动的实际接入点在 `prefrontalCortex/{QwenModel,oNNXModel,model01}.go`，按需填入你的 API Key / 本地模型路径。

### 启动

```powershell
go run .
# 或者先构建
go build -o AgentTest.exe .
.\AgentTest.exe
```

或指定其它配置：

```powershell
$env:AGENTTEST_CONFIG="config/app.custom.yaml"
.\AgentTest.exe
```

### 访问 Web UI

浏览器打开 `http://localhost:8765`：

```
默认用户名：admin
默认密码：见 webui/server.go 常量 loginPass
```

> ⚠️ **WebUI 密码是写死的常量**。**不要把 8765 端口暴露到公网**。生产部署前请改 `webui/server.go` 或加反向代理鉴权层。

### 关闭

终端 Ctrl+C，或终端模式下输入 `exit` / `quit`。

---

## 📖 核心概念

### 五个 Agent

| Key | 文件 | 角色 |
|---|---|---|
| `routerAgent` | `agent/agent/routerAgent.go` | 丘脑：用户/Agent 消息分流，混合路由 |
| `interactiveAgent` | `agent/agent/affectiveInteractiveAgent.go` | 情感对话脑分区，挂 `soul/*.yml` 人格 |
| `behaviorAgent` | `agent/agent/behaviorAgent.go` | 行为编排脑分区，调度 Skill / MCP / Skill Pack |
| `baseAgent` | `agent/agent/baseAgent.go` | 参考实现 / 通用对话 |
| `digitalHumanBehaviorControlAgent` | `agent/agent/digitalHumanBehaviorControlAgent.go` | 数字人本体（骨骼/表情/服装）控制 |

所有 Agent 实现：

```go
type AgentInterface interface {
    Process(ctx context.Context, args ...interface{}) ([]interface{}, error)
    StartListening(ctx context.Context)
    ReportActionResult(skillName string, out []interface{}, err error)
}
```

### Skill 三层树（Domain → Ability → Skill）

`behavior/abilities.yml` 定义对外能力面；`behavior/skill/**/*.go` 注册具体实现。`skill.GlobalManager` 双向链接两者——YAML 里没开放的技能即使代码注册了也调不到。

启动技能用 **结构化 JSON 树**（推荐）：

```json
{
  "tree": {
    "skill": "Generate_Word_Report",
    "children": [
      {"skill": "PowerShell"}
    ]
  },
  "initial_args": ["..."]
}
```

或老字符串 DSL（兼容）：

```json
{"steps": "Generate_Word_Report:1,PowerShell:0", "initial_args": ["..."]}
```

### Soul 人格

`agent/soul/*.yml` 是给 Affective Agent 注入的人格 prompt。当前内置：

- `Nexus.yml` —— 中性、技术导向、无情感
- `KatsuragiMisato.yml` / `KatsuragiMisatoSex.yml` —— 葛城美里风格
- `AsukaLangleySoryu.yml` —— 明日香风格
- 切换：改 `config.paths.soul`

### Memory vs Experience

| 概念 | 文件 | 用途 |
|---|---|---|
| 短期对话历史 | `memory/dialogueHistoryArchiverManager/` | 按 token / 轮数压缩归档 |
| 长期记忆 (RAG) | `memory/myRAGProcessor.go` + `memory/my_agent_memory.jsonl` | 多次召回同一 key 自动固化为"直觉" |
| 经验库 | `experience/myExperienceProvider.go` + `experience.db` | 行为类经验，可被检索/复用 |

### MCP 集成

`capabilities/mcp_runtime.go` 统一启动所有 MCP server，按 `capabilities.attach_to` 决定挂给哪些 Agent。Router 故意不挂（仅做分流）。

随包分发（`mcp_bundled`）支持：把可执行文件放进 `WorkSpace/mcp_bundled/`，`command` 写相对路径即可。

### Skill Pack

外部能力包：放在 `WorkSpace/skill_packs/<pack>/` 下，含 `SKILL.md` 即被自动发现。可选附 `mcp.yaml` 把额外 MCP 合并进运行时。

### Blackboard（神经总线）

集中定义的 topic（见 `body/blackboard/topics.go`）：

| 常量 | 字面量 | 方向 |
|---|---|---|
| `TopicAffectiveDispatch` | `agent.AffectiveInteractiveAgent.dispatch` | Router → Affective（用户原始意图） |
| `TopicAffectiveInput` | `agent.AffectiveInteractiveAgent.input` | Router → Affective（反思转发） |
| `TopicAffectiveOutput` | `agent.AffectiveInteractiveAgent.output` | Affective → Router |
| `TopicBehaviorInput` | `agent.BehaviorAgent.input` | Router → Behavior |
| `TopicBehaviorOutput` | `agent.BehaviorAgent.output` | Behavior → Router |
| `TopicFacadeOutput` | `facadeInteraction.output` | 任意 Agent → 用户门面 |
| `TopicExecStepEvent` / `TopicExecStatus` / `TopicExecResult` | `exec.*` | Skill 执行链上报 |
| `TopicAgentAbort` | `agent.control.abort` | 中止信号 |
| `TopicEnvChange` | `env.change` | 环境/感知层变化 |

所有消息携带：

```go
type Message struct {
    Topic   string
    Payload interface{}
    TurnID  string  // 回合追踪
    Hop     int     // 反思链跳数
}
```

---

## 🔧 项目结构

```text
main.go                          // 启动入口
config/                          // 配置加载与解析
  app.yaml                       // 实际运行配置（.gitignore'd recommended）
  app.example.yaml               // 配置模板

agent/                           // Agent 核心
  agentInterface/                // 统一接口
  agent/                         // 5 个 Agent 实现
  agentManager.go                // 注册中心
  dispatcher/                    // 规则路由（NEW）
  runcontrol/                    // 回合控制 + ctx 元信息
  soul/                          // 人格 YAML

prefrontalCortex/                // 前额叶：CustomExecutor 工具循环 + 模型适配
manager/                         // 模型注册中心

behavior/                        // 行为系统
  abilities.yml                  // Skill YAML 配置
  skill/                         // Skill Go 实现 + Manager
  executor/                      // Skill 树执行引擎

capabilities/                    // 外挂能力层
  mcp_runtime.go                 // MCP 启动 + 工具暴露
  langchain_plugins.go           // 原生 Go 扩展工具
  policy.go / audit.go / obs.go  // 安全 / 审计 / 可观测

skillpacks/                      // 外部 Skill Pack 扫描合并

func/                            // 内部工具（Action 形态暴露给 LLM）
  Router/                        // Agent 间路由工具
  behaveFunc/                    // SetExecutorStep / 注意力 / 经验
  digitalHumanBodyControl/       // 数字人控制
  facadeInteraction/             // 给用户门面投递

memory/                          // 长期 RAG + 对话归档
experience/                      // 经验库 (SQLite + JSONL)

body/                            // 身体层
  blackboard/                    // 进程内 pub/sub + topic 常量
  ears/ eyes/ mouth/             // 设备适配器

agentWorkSpace/
  workSpace/WorkSpace.go         // 主循环 WorkStart
  portal/                        // 终端 / WebUI 统一输出网关
  community/                     // (实验性，未启用)

util/                            // 视觉 / TTS / RAG 公共工具
webui/                           // 内置 Web UI (HTTP + SSE)
Init/                            // 启动期副作用注册
```

---

## ⚙️ 重点配置项（`config/app.yaml`）

| 配置 | 默认 | 作用 |
|---|---|---|
| `agents.default_model` | `deepSeek-onnx` | 全局默认模型 key |
| `agents.rag_recall_threshold` | `3` | RAG 召回 N 次后固化为直觉 |
| `executor.behavior_max_steps` | `40` | Behavior Agent 单次 Run 工具调用上限 |
| `executor.history_tool_rounds_keep` | `12` | 单次 Run 内保留多少轮工具历史 |
| `executor.tool_observation_max_runes` | `16000` | 单条工具返回的 rune 截断 |
| **`executor.router_reflection_max_hops`** | **`2`** | **反思链跳数预算（防死循环）** |
| `web.enabled` | `true` | 是否启动 Web UI |
| `web.listen` | `:8765` | Web UI 监听地址 |
| `capabilities.attach_to` | `[behaviorAgent, interactiveAgent, baseAgent, digitalHumanBehaviorControlAgent]` | 哪些 Agent 挂载 MCP / 扩展工具 |
| `capabilities.mcp.enabled` | `false` | 是否启用 MCP |
| `capabilities.mcp.servers` | `[]` | MCP server 列表（stdio / http） |
| `capabilities.skill_packs.enabled` | `false` | 是否扫描外部 Skill Pack |

---

## 🛡 安全提示

> 这个项目能在你的电脑上**实际操作键鼠、跑 PowerShell、读写文件、发邮件、联网**。请清楚这意味着什么。

- **不要把 WebUI 暴露到公网**。`webui/server.go` 里的密码是常量，没有重置流程。
- **MCP 与 Skill Pack 是外部代码**。`capabilities.security.allow_mcp_server_names` 与 `deny_tool_name_substrings` 可以做白/黑名单。
- **PowerShell Skill 默认工作目录是 `paths.workspace`**（`WorkSpace/`），用相对路径可以避免误触全盘。
- **`config/app.yaml` 不要入库**。建议从 `app.example.yaml` 复制一份本地配置，并把 `app.yaml` 加入 `.gitignore`。

---

## 🧪 开发提示

- 编译验证：`go build ./...`
- 静态检查：`go vet ./...`（仓库有几条历史告警与本架构无关）
- 项目目前**无自动化测试**，对 `agent/dispatcher`、`agent/runcontrol` 跳数逻辑、`behavior/executor` SkillStep 解析手工验证为主。
- 黑板调试：所有跨 Agent 消息打印 `turn=... hop=...`，配合 stdout 即可追踪。

---

## 🙏 致谢 & 依赖

- [langchaingo](https://github.com/tmc/langchaingo) —— Agent / Tool 抽象
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) —— MCP 客户端
- [robotgo](https://github.com/go-vgo/robotgo) —— 键鼠自动化
- [pion/mediadevices](https://github.com/pion/mediadevices) —— 摄像头/麦克风
- [gen2brain/malgo](https://github.com/gen2brain/malgo) —— 音频 IO
- [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai) —— OpenAI 兼容协议
- [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) —— 本地经验库

---

## 📄 License

待定 / TBD —— 在公开发布前请补一个明确 License（MIT / Apache-2.0 / GPL 任选）。
