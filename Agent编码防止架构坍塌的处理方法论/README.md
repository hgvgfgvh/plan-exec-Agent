# Agent 编码防止架构坍缩 — 方法论文档集

本目录存放 **人工设计意图** 与 **对照当前代码库同步得到的架构/验收文档**，供编码 Agent 在修改 `AgentTest` 前阅读。  
**不替代** 仓库根目录 `AGENTS.md`（后者为运行时能力目录，由 `capabilities.BuildAgentCatalogMarkdown()` 动态注入）。

## 文档地图

| 文件 | 维护者 | 用途 |
|------|--------|------|
| [METHODOLOGY.md](./METHODOLOGY.md) | 人工 | Harness 四层护栏模型与使用方式 |
| [DESIGN_INTENT.md](./DESIGN_INTENT.md) | **人工**（宪法） | **阶段一～三铁律**、设计意图与决策历史 |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | 人工确认 + Agent 可草拟同步 | §1～13 现网；§14 阶段二已实现；§15 阶段三（Soul MCP 规划） |
| [ACCEPTANCE_RULES.md](./ACCEPTANCE_RULES.md) | 同上 | 验收规则（§I 阶段二；§J 阶段三规划） |
| [ARCHITECTURE_DRIFT.md](./ARCHITECTURE_DRIFT.md) | 人工决策队列 | 设计意图 vs 实现的差异分类 |
| [GOLDEN_RULES.md](./GOLDEN_RULES.md) | 事故沉淀 | 高信号强规则（须尽快编码为测试/lint） |

## 编码 Agent 开始前（最小流程）

1. 读 `DESIGN_INTENT.md` → 明确不可破坏的边界  
2. 读 `ARCHITECTURE.md` + `ACCEPTANCE_RULES.md` → 了解实现与验收  
3. 若有未决差异，读 `ARCHITECTURE_DRIFT.md`  
4. 修改后运行 `go test ./...`（尤其 `plan/verify`、`lintcheck`、`plan/todolist`）  
5. 坍缩风险自查见 `METHODOLOGY.md` 文末清单  

## 与仓库其它文档的关系

- `AGENTS.md`：MCP / 内置技能 / 外挂 SKILL 的**第一层能力目录**（运行时）  
- `config/app.yaml`：路径、executor 记忆与步数等**可执行配置**  
- 本目录：Plan/Exec（阶段一）、Memory MCP / Exec-Simple（阶段二）、Soul MCP / 人格钩子（阶段三规划）、TodoList、Gate 的**设计意图与架构承诺**  
- `AgentTestSoulMCP/docs/`：Soul 子系统宪法（与 `AgentTestMemoryMCP` 同级外置仓库）  
- 阶段二 CLCA 设计流程：`.cursor/skills/agent-clca-design-zh`  

## 阶段速览

| 阶段 | 编排形态 | 代码状态 |
|------|----------|----------|
| **一** | `plan → 逐步 exec` + TodoList + Gate | **已实现**（主路径） |
| **二** | `plan → Memory MCP → exec-simple \| exec` + skill 候补 | **主路径已实现**（`plan/memoryhook` + `execSimpleAgent` + 降级）；pitfall 语义 / Skill 沉淀 / 端到端集成测试仍待补 |
| **三** | `soul_retrieve` → `memory_retrieve` → Plan；`soul_store` 并行 | **规划/文档**（`plan_soul_hook` + `AgentTestSoulMCP`）；代码未实现 |

## 同步说明

- 本文档集生成于 **2026-05-19**，依据当时 `main` 工作区代码梳理阶段一。  
- **2026-05-19**：已同步 `plan/verify/gate.go` — `layer2AuditEnabled=false`（L2 临时关闭）。  
- **2026-05-20**：阶段二设计意图并入 `DESIGN_INTENT.md`（铁律 F2-1～F2-9）；`ARCHITECTURE.md` §14、`ACCEPTANCE_RULES.md` §I、`ARCHITECTURE_DRIFT.md` DI-9～DI-13。  
- **2026-05-24**：对照代码确认阶段二主路径已落地 — `mcp_provider.go`（retrieve/store）、`DecideRoute`、`runExecSimpleEpisode`、门户 `StoreTurnAfterProcess`；更新 DRIFT DI-9～DI-11 与 §I 验收状态。  
- **2026-05-24**：阶段三 Soul MCP 设计入宪 — `DESIGN_INTENT` 阶段三铁律 F3-1～F3-9、`ARCHITECTURE` §15、`ACCEPTANCE` §J、DRIFT DI-14～DI-16；`AgentTestSoulMCP/docs` 初版（无代码）。  
- 后续：补 Memory↔Simple 端到端测试、显式 pitfall store、Skill 候补；阶段三按 Soul 文档实现 Host 钩子与 MCP；**不得用现网代码覆盖宪法**；未实现项保留在 DRIFT。  
