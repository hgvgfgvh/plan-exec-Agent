# 验收规则

本文将 **设计意图中的架构承诺** 转为可检查项，并索引仓库内**已有**自动化测试。  
新功能或 Agent 改动后应满足下列规则；无法满足时记入 `ARCHITECTURE_DRIFT.md` 并人工决策。

---

## A. 角色与边界

| ID | 规则 | 验证方式 |
|----|------|----------|
| A1 | 用户主入口经 `RunRouterTurn` 进入 `planAgent`（已注册时） | 代码审查 `agentWorkSpace/portal/gateway.go` |
| A2 | PlanAgent **不**挂载 MCP/技能执行工具 | `NewPlanAgent` 无 capabilities 注册；Plan 仅 `chatJSON` |
| A3 | Exec 能力来自 `capabilities.attach_to: behaviorAgent` | `config/app.yaml` + `behaviorAgent` 初始化 |
| A4 | Plan 编排循环由 Go `for` 驱动，非 Plan LLM ReAct 逐步调度 | `planAgent.Process` 结构 |

---

## B. 五层上下文

| ID | 规则 | 验证方式 |
|----|------|----------|
| B1 | Plan 能力注入仅为名称级概览，不含工具 Schema | `BuildPlanCapabilityOverview` 文案与实现 |
| B2 | Exec system 含 `FormatCatalogForExecutor` + 可选 `get_capability_details` | `customExecutor.go` |
| B3 | Plan 跨轮记忆使用独立 `sessionmemory.Manager`，与 Behavior `ChatHistory` 分离 | `sessionmemory` 包注释与构造 |
| B4 | Plan 步 Exec **步开始**清空 `ChatHistory` | `behaviorAgent.go` + `IsPlanStepExecution` |

---

## C. TodoList 控制台

| ID | 规则 | 验证方式 |
|----|------|----------|
| C1 | 每次 `Process` 创建新 `Document` 并 `Save` 至 `WorkSpace/ToDoList/{id}.json` | `planAgent.Process` 开头 |
| C2 | 调度/结果/调节/升级写入 `AppendFeedback` | `planAgent.go` 各 phase |
| C3 | 步骤完成写入 `RecordStepOutcome`（摘要、产物、工具） | `StepCompleted` 分支 |
| C4 | 下一步命令含已完成路标 `【已完成步骤路标】` | `buildStepCommand` + `roadmap_test.go` |
| C5 | Plan 步禁止 Exec 使用 `update_task_dashboard` | `buildStepCommand` 文案 + executor 过滤 |

---

## D. 结构化回执

| ID | 规则 | 验证方式 |
|----|------|----------|
| D1 | Plan 单步结束**必须**调用 `report_step_result` | `classifyStepOutcome` 无 report → fail |
| D2 | `status=ok` 触发 `verify.Gate` | `classifyStepOutcome` |
| D3 | 工具调用自动记入 `RecordPlanToolCall`，合并进 `StepReport.ToolsCalled` | `action_execute.go` + `step_report.go` |
| D4 | `SetExecutorStep` 后须 reconcile 真实技能结果再标 ok | `planstep/reconcile_test.go` |

---

## E. Verification Gate（已实现部分）

> **临时状态（2026-05-19）**：`plan/verify/gate.go` 中 `layer2AuditEnabled = false`，**Layer 2 行为审计默认通过**，仅 **Layer 1（验盘）** 生效。  
> 原因：L2 用工具名启发式 +「有 artifacts 须有 write」与路标/handoff 语义冲突，多样 MCP/只读确认步易误杀。恢复前见 `gate.go` 常量注释与 `ARCHITECTURE_DRIFT.md` DI-4。

| ID | Tier | 规则 | 自动化 | 当前 |
|----|------|------|--------|------|
| E1 | 1+ | `status=ok` 且无 artifacts 时，Gate 可通过（无路径则跳过 L1 文件校验） | `TestGate_Tier1_AllowsNoToolCalls` | **生效** |
| E2 | 2+ | `status=ok` 且未记录工具调用 → Gate 失败 | `TestGate_Tier2_RequiresToolCalls` | **暂停**（L2 关闭后通过） |
| E3 | 2+ | 声明 `artifacts` 但无 write/edit/SetExecutorStep 类工具 → 失败 | `TestGate_Tier2_ArtifactsNeedWriteTool` | **暂停** |
| E4 | 1+ | 声明的 artifact 路径：文件存在、非占位、trim 后 ≥1 rune | `plan/artifact/validate.go` | **生效** |
| E5 | 2+ | 单工具调用 >6 次 → 死循环失败 | `gate.go` `auditToolBehavior` | **暂停** |

**恢复 L2 时的目标契约（未编码）**：写步调用 write 类工具须声明 `artifacts`；读/handoff 步允许 `artifacts` 引用已有文件且不要求 write；`hasWriteLikeTool` 需按能力类型而非子串。

**设计意图未覆盖（见 DRIFT）**：Tier 3 独立 L3 judge；L1 显式退出码/linter；品质不达标 vs 客观失败分流。

---

## F. Tier

| ID | 规则 | 验证方式 |
|----|------|----------|
| F1 | 纯归纳/勿调工具步强制 tier=1；否则模型 tier∈[1,3] 或 `stepmeta.InferTier` | `stepmeta.ResolveTier` + `sanitizeDocumentTiers` |
| F1b | instruction 含「勿再调用」「基于上一步归纳」且模型写 tier=2 | 落盘 tier→1；L2 恢复后 Gate 不因空 `tools_called` 失败（**当前 L2 关闭，tier=2 亦无 tools 审计**） |
| F2 | tier 传入 `verify.Gate(rep, stepTier)` | `classifyStepOutcome` |
| F3 | tier≤1 的 step command 允许 `tools_called: []` 示例 | `buildStepCommand` |

**设计意图未覆盖**：Exec 单向上调 tier；Gate 失败后 tier 抬高重验。

---

## G. 防坍缩静态规则

| ID | 规则 | 自动化 |
|----|------|--------|
| G1 | Plan 路径禁止用户原文关键词/正则分流（`Classify`、寒暄规则路由等） | `lintcheck/no_rule_routing_test.go` |
| G2 | 禁止复活 `TierTrivial` / `LightStep` 等旧规则类型名 | 同上 forbidden 列表 |

---

## H. 建议补充的架构验收测试（尚未实现）

> 自 `DESIGN_INTENT.md` 基石节点推导，供后续迭代编码。

| 基石 | 可观测规律 | 建议测试 |
|------|-----------|----------|
| Plan 跨轮记忆 | N 轮 `RecordTurn` 后 `plan_memory` JSONL 有写入或 `Retrieve` 命中 | `plan/sessionmemory` 集成测试 |
| 步间记忆隔离 | 连续两 Plan 步：第二步 `ChatHistory` 不含第一步 tool 消息 | mock Memory 断言 |
| 路标不重复劳动 | 第二步 prompt 含第一步 `ResultSummary` | `plan_step_command_test` 扩展 |
| Tier 只升不降 | Exec 上报更高 tier 后 Gate 用新 tier | 待 API 设计后补 |

---

## I. 阶段二（Memory + Simple · 2026-05-24 同步代码）

> 宪法：`DESIGN_INTENT.md` 阶段二铁律 F2-1～F2-9。  
> 已有单元测试：`plan/memoryhook/*_test.go`、`agent/agent/plan_exec_simple_test.go`；**缺** 真实 Memory MCP 端到端集成测试。

| ID | 规则 | 状态 |
|----|------|------|
| I1 | Plan 拆步后调用 Memory retrieve 路由；未命中不得伪造 simple | **已实现**：`DecideRoute` → `MCPProvider.Retrieve`；noop/未命中 → 保守 Exec |
| I2 | 高 tier / 低 confidence **不得**走 Exec-Simple | **已实现**：`exec_simple_max_tier` + `exec_simple_min_confidence` + Memory matched |
| I3 | Simple 路径使用 TodoList-simple | **已实现**（形态演进）：`execution_mode: simple` + 初始拆步链整份下发；非 Memory 单独压缩 JSON |
| I4 | Exec-Simple episode 内不逐步 report；仅 episode 结束回传 | **已实现**：`buildSimpleEpisodeCommand` 约束 + `classifySimpleOutcome` |
| I5 | Simple 成功回传含摘要 + tools 迹；Plan 合流前 Gate | **已实现**：`StepReport` + `verify.Gate`（当前 L1；L2 仍关闭） |
| I6 | Simple 失败须 pitfall store + 新 TodoList + Exec 逐步 | **部分实现**：降级 + 新 TodoList + Exec **已实现**；显式 pitfall **未**（通用 `StoreTurnAfterProcess` 含失败终态） |
| I7 | simple 不可用/降级时阶段一行为不变 | **已实现**：`runConservativeExecLoop` 保留 |
| I8 | Skill 沉淀不替代 Memory 主闭环 | **未实现** |
| I9 | Plan 不挂载执行类 MCP | **已实现**：Plan 仅 `chatJSON` + hook；Memory 经 Host `MCPProvider` |

**建议补测**：

- 集成：真实 `memory-mcp.exe` + factworld → 第二次同类任务走 simple 且 `dispatches` 更少  
- 集成：simple 失败 → 断言新 TodoList id 后缀 `-exec`、`execution_mode=exec`  
- 契约：Memory MCP retrieve/store hints / `---memory-route---` 块快照测试

---

## J. 阶段三（Soul MCP + Host 钩子 · 规划 · 2026-05-24）

> 宪法：`DESIGN_INTENT.md` 阶段三铁律 F3-1～F3-9。  
> 子系统验收细则：`AgentTestSoulMCP/docs/ACCEPTANCE_RULES.md`。  
> **当前状态**：全部 **未实现**；本节为实现前冻结清单。

| ID | 规则 | 状态 |
|----|------|------|
| J1 | 回合开始 `soul_retrieve` 在 `memory_retrieve` **之前** 注入 | **未实现** |
| J2 | `soul_store` 仅消费 WebUI 对话序列化；不消费 Behavior 步内 tool 观测 | **未实现** |
| J3 | Soul retrieve **不含** `exec_simple_match` / Memory 路由块 | **未实现** |
| J4 | Soul MCP 对外仅 `soul_store` / `soul_retrieve` | **未实现** |
| J5 | `soul_store` 失败不阻断门户回复；`soul_retrieve` 失败降级为空 hints | **未实现** |
| J6 | `soul.config` 基座只读；动态改写仅 `soul_overlay` 且可审计 | **未实现** |
| J7 | 人格/议题 **不**写入 Memory `facts.jsonl` 主路径 | **未实现**（依赖 Host+双 MCP 隔离） |
| J8 | Plan 不直接挂载 Soul 工具（经 Host hook） | **未实现** |
| J9 | 跨会话：昨日议题今日首句可 retrieve 命中（人工场景测试） | **未实现** |

**建议首测场景**：

- 两轮 WebUI：第一轮讨论「项目 X + 论文 Y」；第二轮仅问「昨天那个论文的结论？」→ retrieve 含 `event_context` 指代 X/Y  
- 用户固定口头禅/称呼 → 第二轮 Plan 回复风格一致（人工抽检 + profile 条目快照）  
- Soul MCP 宕机 → Plan 仍正常；无 soul hints  

---

## 验收命令

```bash
go test ./plan/verify/... ./plan/todolist/... ./lintcheck/... ./agent/agent/... ./plan/planstep/...
go test ./...
```

---

## Gate 失败后的 Plan 行为（当前）

| Gate/回执情况 | Plan 行为 |
|---------------|-----------|
| 无 `report_step_result` | `StepFailed`，摘要说明未提交 |
| `status=fail` | `StepFailed` |
| `status=ok` + Gate 失败 | `StepFailed`，摘要含 Gate failures |
| 连续失败 ≥ `planMaxAdjustPerStep` | `escalateToUser`，计划 `blocked` |

**设计意图中的细分**（行为审计 → replan、品质 → 改进重试）在代码中**尚未**按 Gate `Layer` 分支，统一走 `adjustPlanAfterFailure`。
