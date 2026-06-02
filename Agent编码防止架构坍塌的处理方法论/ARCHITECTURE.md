# 架构文档

- **§1～§13**：阶段一，**对照当前代码**（2026-05-19 起，含 Gate L2 临时关闭说明）。  
- **§14**：阶段二 **Memory + Simple 主路径**（2026-05-20 宪法，**2026-05-24 代码已落地**；pitfall 语义 / Skill 候补见 DRIFT）。

模块路径均相对于仓库根目录。

---

## 1. 总览（阶段一 · 已实现）

```text
用户输入
    │
    ▼
agentWorkSpace/portal/gateway.go :: RunRouterTurn
    │
    ▼
PlanAgent.Process          ← 编排循环（代码驱动，非 LLM ReAct 拆步）
    │   ├─ sessionmemory     ← Plan 跨轮记忆
    │   ├─ todolist          ← WorkSpace/ToDoList/*.json
    │   └─ 每步 ─────────────► BehaviorAgent.Process (Exec)
    │                              ├─ CustomExecutor.Run (ReAct)
    │                              ├─ capabilities / MCP / SKILL
    │                              └─ report_step_result → verify.Gate
    ▼
用户可见回复 + TodoList 持久化
```

| 角色 | 类型 | 入口 |
|------|------|------|
| PlanAgent | `agent/agent/planAgent.go` | `Process(ctx, query)` |
| ExecAgent | `agent/agent/behaviorAgent.go` | `Process(ctx, cmd)` |
| 能力目录（Exec） | `capabilities/agent_catalog.go` | 注入 `CustomExecutor` system |
| 能力概览（Plan） | `capabilities/plan_catalog.go` | `BuildPlanCapabilityOverview()` |

---

## 2. 五层上下文 — 实现映射

代码中**无**名为 `BackgroundLayer` 的类型；五层由 **不同注入点** 实现。

### 2.1 PlanAgent

| 层 | 实现 | 关键符号 |
|----|------|----------|
| 背景 | RAG + archiver + 经验 | `plan/sessionmemory.Manager.PrepareUserContext` |
| 能力 | MCP/内置/外挂 **名称** | `capabilities.BuildPlanCapabilityOverview` |
| 状态 | TodoList 步骤与状态 | `todolist.FormatForPrompt`（**调节/合成**路径） |
| 反馈 | 步骤 Feedback 链 | `todolist.Document.AppendFeedback` |
| 需求 | 用户诉求 | `doc.UserRequirement`，prompt 中指代解析说明 |

组装：`PlanAgent.chatJSON` → `Session.BuildMessages(system, user, ctxBlock)`。

**配置**（`config/app.yaml`）：

- `paths.plan_memory` → `memory/plan_agent_memory.jsonl`
- `executor.plan_max_history` / `plan_archive_rounds` / `dialogue_archive_tokens`

### 2.2 ExecAgent（BehaviorAgent + CustomExecutor）

| 层 | 实现 | 关键符号 |
|----|------|----------|
| 背景 | 灵魂/本能；Plan 步**无**跨步 ChatHistory | `LongTermMemory.GetInstinct`；`ChatHistory.Clear` when `IsPlanStepExecution` |
| 能力 | L1 目录 + L2 `get_capability_details` | `FormatCatalogForExecutor`、`capability_details_tool` |
| 状态 | 单步命令 + 路标 | `buildStepCommand`、`todolist.FormatRoadmapForExec` |
| 反馈 | 步内工具观测 | ReAct `history`，`trimExecutorHistoryTail` |
| 需求 | 总需求 + 本步任务 | `buildStepCommand` 内 `用户总需求` / `本步任务` |

**配置**：

- `executor.history_tool_rounds_keep`（步内 ReAct 尾保留）
- `executor.behavior_max_steps` / `behavior_max_history`（非 Plan 步 Behavior）
- `capabilities.attach_to: behaviorAgent`（MCP 挂在 Exec）

---

## 3. PlanAgent 编排循环

**文件**：`agent/agent/planAgent.go`

```text
Process
  → NewID + generateInitialPlan (LLM JSON → steps + tier)
  → todolist.Save
  → loop while PlanActive:
        NextPending → StepRunning
        buildStepCommand → AppendFeedback(dispatch)
        WithPlanStepExecution → Executor.Process(cmd)
        classifyStepOutcome → verify.Gate (if report ok; 当前 L1 only)
        StepCompleted → RecordStepOutcome + Save
        StepFailed → adjustPlanAfterFailure / escalateToUser
  → buildUserFacingReply + Session.RecordTurn
```

**约束常量**：

- `planMaxDispatchPerTurn = 40`（单用户回合最多下发步数）
- `planMaxAdjustPerStep = 3`（单步失败调节次数）
- `planMaxStepsPerPlan = 24`

**编排隔离**：`runcontrol.BeginPlanOrchestration` / `EndPlanOrchestration` — 抑制编排期间异步 Behavior 反馈干扰。

**PlanAgent 无工具**：`NewPlanAgent` 不注册 MCP；计划生成/调节为 **单次 LLM JSON**，非工具循环。

---

## 4. TodoList 控制台

**包**：`plan/todolist`

| 概念 | 说明 |
|------|------|
| 路径 | `{workspace}/ToDoList/{id}.json`，`id = NewID(userRequirement)` |
| 结构 | `Document`：需求、摘要、计划状态、`[]Step` |
| Step 字段 | `id, title, instruction, capability_hints, tier, status, attempts, result_*, artifacts, tools_called, feedback` |
| 路标 | `FormatRoadmapForExec(doc, currentIdx)` — 仅 `completed`/`skipped` 且 index < current |
| 结果落盘 | `RecordStepOutcome` — 供下一步路标与 JSON 持久化 |

**Feedback phase**：`create | dispatch | result | adjust | escalate | validate`

**能力名校验**：`sanitizeDocumentCapabilityHints`（调节后移除未知能力名）

---

## 5. Exec 单步执行与记忆清零

**触发**：`runcontrol.WithPlanStepExecution(ctx)`（`planAgent.go` 下发前）

**BehaviorAgent.Process**（约 146–155 行）：

1. 可选 `ZeroState` 时清空记忆（`agents.behavior_zero_state`）  
2. **Plan 步必做**：`ChatHistory.Clear` + `ClearStepReport(turnID)`  
3. `CustomExecutor.Run` — ReAct，Plan 步过滤 `update_task_dashboard`，限制 batch 工具数  
4. 步末 `planstep.ReconcileSkillStepAfterRun` — 补齐 `SetExecutorStep` 异步技能结果  

**步内记忆**：ReAct `history` 仍累积至本步结束；**步间**不保留。

---

## 6. Step Descriptor（`buildStepCommand`）

注入内容（顺序要点）：

1. 计划 ID、步骤序号、`tier` 说明  
2. 标题、`instruction`、`capability_hints`  
3. `【已完成步骤路标】`（若有）  
4. `用户总需求`  
5. tier≤1 / tier>1 不同约束与 `report_step_result` 示例  
6. 禁止 `update_task_dashboard`、禁止多步 ToolPlan JSON  

---

## 7. 结构化回执与工具埋点

| 组件 | 职责 |
|------|------|
| `func/behaveFunc/report_step_result.go` | Exec 必填 JSON 回执 |
| `plan/planstep/reconcile.go` | 技能步结果与 report 对齐 |
| `agent/runcontrol/step_report.go` | `RecordPlanToolCall`、`SetStepReport`、`TakeStepReport` |
| `prefrontalCortex/action_execute.go` | 每次工具调用 `RecordPlanToolCall` |

`classifyStepOutcome`：Plan 步必须从 `TakeStepReport` 取回执；`status=ok` 时调用 `verify.Gate`。

---

## 8. Tier 分配（当前实现）

**文件**：`planAgent.go` — `inferStepTier`、`clampPlanTier`

| 规则 | 结果 tier |
|------|-----------|
| 无 hints 且 instruction 含「无需调用工具」类短语 | 1 |
| title/instruction 含 重构/安全/支付/… 等关键词 | 3 |
| 其它 | 2 |

- 模型 JSON 中 `tier` 若 ∈ [1,3] → **原样采用**（`clampPlanTier` 不抬高）  
- 无效 tier → `inferStepTier` 填充  

**写入**：`todolist.Step.Tier` → `buildStepCommand` → `verify.Gate(rep, stepTier)`

---

## 9. Verification Gate（当前实现）

**包**：`plan/verify/gate.go` + `plan/artifact/validate.go`

**开关**：`layer2AuditEnabled`（当前 **`false`**，L2 临时默认通过）。

```text
status != ok  → Gate 直接 Passed（由 classify 走 fail 分支）

Layer 1 (hard_rules) — 始终执行（所有 tier 的 status=ok）:
  artifact.ValidateReportArtifacts（仅当 artifacts 非空）
    - 路径可读（resolveArtifactPath）
    - 非占位（skillwait.IsPlaceholderSkillSummary）
    - 内容 trim 后 ≥ 1 rune（非空，挡 0 字节/纯空白）

tier <= 1 → L1 通过后即 OK

tier >= 2 → Layer 2 (behavior) — 当前跳过:
  [未启用] tools_called 非空
  [未启用] 单工具重复 > 6 次 → 死循环
  [未启用] 有 artifacts 须有 write/edit/SetExecutorStep 类工具
  → 实现仍保留于 auditToolBehavior，恢复时设 layer2AuditEnabled=true
```

**L2 临时关闭原因（遗留，见 gate.go 注释）**：

- `artifacts` 在路标/prompt 中为「供下一步读取」，L2 却要求「本步须有 write 类工具」→ 只读确认步误杀。  
- `hasWriteLikeTool` 仅靠工具名字符串，无法覆盖多样 MCP/技能命名。  

**当前未实现**：Layer 3 LLM judge、Approval Gate、tier 3 与 tier 2 的差异化；L2 契约对齐后重新启用。

**失败处理**（`classifyStepOutcome`）：Gate 不通过（**当前仅 L1**）→ `StepFailed` + 失败摘要 → `adjustPlanAfterFailure` 或 `escalateToUser`。

---

## 10. 计划调节与升级

| 函数 | 行为 |
|------|------|
| `adjustPlanAfterFailure` | LLM 输出 retry / skip / replace steps |
| `escalateToUser` | 超 `planMaxAdjustPerStep` 阻塞计划，返回用户消息 |

---

## 11. 静态护栏（与架构相关）

| 位置 | 作用 |
|------|------|
| `lintcheck/no_rule_routing_test.go` | 禁止 Plan 路径用户原文关键词/正则分流 |
| `plan/verify/gate_test.go` | Gate 契约（L2 关闭时 tier2 用例仅断言通过） |
| `plan/todolist/roadmap_test.go` | 路标格式 |
| `agent/agent/plan_step_command_test.go` | 命令注入字段 |

---

## 12. 配置速查

| 键 | 用途 |
|----|------|
| `paths.workspace` | TodoList 根 |
| `paths.plan_memory` | Plan 长效记忆 |
| `executor.plan_max_history` | Plan buffer 轮数 |
| `executor.plan_archive_rounds` | Plan 压缩保留尾 |
| `executor.history_tool_rounds_keep` | Exec ReAct 尾 |
| `executor.plan_step_max_steps` | Plan 单步 ReAct 上限（`config.go` 默认 8，可不在 app.yaml） |
| `agents.behavior_zero_state` | 每次 Behavior 调用清空记忆（与 Plan 步清记忆独立） |

---

## 13. 模块依赖方向（简图）

```text
portal/gateway
    → agent/agent (PlanAgent, BehaviorAgent)
        → plan/todolist, plan/sessionmemory, plan/verify, plan/planstep
        → prefrontalCortex (CustomExecutor)
        → capabilities (catalog, MCP, tools)
        → func/behaveFunc (report_step_result, SetExecutorStep, …)
        → agent/runcontrol
```

Plan **不** import `prefrontalCortex` 工具执行链；Exec **不** 写 TodoList 文件（仅 report + 代码更新）。

---

## 14. 阶段二架构（主路径已实现 · 2026-05-24 同步代码）

> 宪法：`DESIGN_INTENT.md` 阶段二铁律 F2-1～F2-9、§10～§16。  
> 漂移登记：`ARCHITECTURE_DRIFT.md` DI-9～DI-13（pitfall 语义 / Skill 候补仍待补）。

**已实现（主路径）**：

| 组件 | 路径 |
|------|------|
| Memory MCP 客户端 | `plan/memoryhook/mcp_provider.go`（`provider: mcp` → `memory_retrieve` / `memory_store`） |
| 回合 retrieve/store | `portal/gateway.go` → `RetrieveTurnBeforeProcess` / `StoreTurnAfterProcess` |
| Plan 路由 | `plan/memoryhook/memory_mcp_hook.go` → `DecideRoute`（tier + confidence + matched） |
| Exec-Simple Agent | `agent/agent/execSimpleAgent.go`（`execSimpleAgent`） |
| Plan 下发 | `planAgent.runExecSimpleEpisode` / `buildSimpleEpisodeCommand` |
| Episode 回传与 Gate | `classifySimpleOutcome` + `verify.Gate` + `applySimpleSuccess` |
| Simple 失败降级 | `runExecSimpleEpisode` → 新 TodoList → `runConservativeExecLoop` |
| 配置 | `plan_memory_hook.*`、`executor.exec_simple_*`、`capabilities.attach_to` 含 `execSimpleAgent` |

### 14.1 目标总览

```text
用户输入
    │
    ▼
PlanAgent.Process
    ├─ Memory MCP.retrieve（结构化经验 / 成功路径）
    ├─ 路由判定 ─────────────────────────────────────────┐
    │     命中 + 把握度足够                               │ 未命中 / 复杂 / 无把握
    │         ▼                                           ▼
    │   TodoList-simple                          TodoList（阶段一）
    │         ▼                                           ▼
    │   Exec-Simple（episode 持续执行）              Exec 逐步（§1 现网）
    │         │                                           │
    │         └─ 成功 / 路径失败 ──► Plan ◄────────────────┘
    │                   │
    │         失败 ──► memory store(pitfall) + 新 TodoList → Exec
    │
    ├─ Memory MCP.store（成功强化 / 失败 pitfall）  ← 优化流
    └─ skill沉淀（候补，非主路径）
```

### 14.2 与阶段一对比

| 维度 | 阶段一（现网） | 阶段二（目标） |
|------|----------------|----------------|
| Plan→执行粒度 | 每 Step 一次 Exec | 可增加 **每 episode 一次** Exec-Simple |
| 记忆 | `sessionmemory` + `plan_memory` JSONL（对话型） | **+** 外置 Memory MCP（路径/工具链/产物结构） |
| 提速机制 | 路标注入、少步拆计划 | **复用已验证路径** → TodoList-simple |
| 失败策略 | adjust / escalate | **+** 降级回阶段一 Exec |
| 验收 | 每步 Gate（当前 L1） | Simple 回传 Plan 时 **episode 级** 验收合流（细则待冻结） |

### 14.3 模块状态（2026-05-24）

| 组件 | 建议职责 | 状态 |
|------|----------|------|
| Memory MCP 客户端 | `RegisterProvider("mcp")` + `plan_memory_hook.provider: mcp` | **已实现** |
| OnTurnRetrieve / OnTurnStore | 回合前 hints 注入；回合后异步 `memory_store` | **已实现** |
| Plan simple 路由 | Memory hook 命中 + confidence + max tier；无命中不走 simple | **已实现** |
| `TodoList-simple` | `execution_mode: simple` + 整份初始拆步链下发 | **已实现**（形态：非 Memory 单独压缩 JSON，见 DI-10） |
| Exec-Simple | `runcontrol.WithPlanSimpleExecution` + 独立 Executor 步数/历史上限 | **已实现** |
| Episode 结构化回传 | `report_step_result` + `StepReport` + episode 级 Gate | **已实现** |
| Simple 失败 pitfall | 显式 `kind=pitfall` 或等价 API | **部分实现**（通用 turn store 含失败终态；无 dedicated pitfall） |
| Skill 沉淀流水线 | 成功 episode → skill pack 草稿 | **未实现** |

### 14.4 阶段一代码在阶段二中的位置

- **保留**：`planAgent.Process` 的 exec 分支、`buildStepCommand`、`verify.Gate`、`lintcheck`。  
- **已扩展**：`Process` 拆步后 `DecideRoute`；命中则 `runExecSimpleEpisode`；失败则新建 TodoList 走逐步 Exec；门户层 `StoreTurnAfterProcess` 写入 Memory MCP。  
- **不改动原则**：Plan 仍不直接挂载执行类 MCP（F2-9）；Exec 逐步路径行为与 §1～§13 一致。

### 14.5 与 CLCA / 外置仓库

- 角色划分与 `.cursor/skills/agent-clca-design-zh` 一致：Plan、Exec、Exec-Simple、Memory 服务。  
- Memory MCP 若独立 repo（如 `AgentTestMemoryMCP`），其 `DESIGN_INTENT.md` 为 Memory 子系统宪法；本目录宪法约束 **Host/Plan 如何调用**，不重复定义 MCP 内部 schema。

---

## 15. 阶段三架构（Soul MCP + Host 钩子 · 规划未实现 · 2026-05-24）

> 宪法：`DESIGN_INTENT.md` 阶段三铁律 F3-1～F3-9、§17～§22。  
> 子系统 SSOT：`C:\DATA\GODATA\AgentTestSoulMCP\docs\`。  
> 漂移：`ARCHITECTURE_DRIFT.md` DI-14～DI-16。

### 15.1 目标总览

```text
用户输入（WebUI）
    │
    ▼
portal.RunRouterTurn
    ├─ soul_retrieve（人格 + 议题/事件）     ← plan_soul_hook · 先于 Memory
    ├─ memory_retrieve（执行经验 / simple 路由）
    ├─ PlanAgent.Process / Affective 等
    │       └─ Behavior 步内：不挂 Soul 工具
    │
    └─ 回合结束
            ├─ soul_store（异步 · WebUI 对话 episode）
            └─ memory_store（异步 · 执行经验）
```

### 15.2 与 Memory MCP 对比

| 维度 | Memory MCP | Soul MCP |
|------|------------|----------|
| 问题域 | 怎么干、失败路径、工具链 | 在聊什么、用户是谁、怎么说话 |
| 工具 | `memory_store` / `memory_retrieve` | `soul_store` / `soul_retrieve` |
| Host 钩子 | `plan_memory_hook` | `plan_soul_hook`（规划） |
| 路由 | 可输出 simple 路由块 | **禁止** 参与 Exec-Simple |
| 配置 | `mcp_env` + LLM 抽取 | `soul.config` + 可选 overlay |
| 仓库 | `AgentTestMemoryMCP` | `AgentTestSoulMCP` |

### 15.3 模块状态（规划）

| 组件 | 建议路径 | 状态 |
|------|----------|------|
| `plan/soulhook` | 对称 `plan/memoryhook` | **未实现** |
| `portal` retrieve/store | `InjectTurnHints` 前插入 Soul | **未实现** |
| `plan_soul_hook` 配置 | `config/app.yaml` | **未实现** |
| Soul MCP 进程 | `AgentTestSoulMCP` | **仅文档** |

### 15.4 注入契约（草案）

Retrieve 返回 Host 可拼入 `planInput` 的块（名称实现前冻结）：

- `persona_prompt`：基座 `soul.config` + profile 条目 + 可选 overlay  
- `event_context`：议题/项目/论文等 **可指代** 事件列表（带 `last_mentioned`）  
- `retrieve_meta`：预算、命中数、降级原因（可选）

Store 入参：

- `context`：WebUI 整轮或增量对话（与 Memory store 同源序列化器，**不同**下游 schema）  
- `user_id` / `session_id`（多租户待冻结）

### 15.5 与阶段一/二关系

- **保留**：`sessionmemory`、`plan_memory_hook`、Exec-Simple、F2 全部铁律。  
- **新增**：Soul 为 **体验层** 并行轨；失败时 **静默降级**（与 Memory hook 一致）。  
- **不改动**：Plan 仍不直接挂载 Soul 工具；Exec 仍步间清记忆。
