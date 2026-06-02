# 黄金法则

> 来自**真实事故**或高风险模式的短规则。每条须含「执行方式」；**7 日内**落地为 `go test` 或 `lintcheck`，否则标为「建议」而非法则。

---

## GR-001 — Plan 路径禁止用户原文规则分流

**事故来源**：历史实现曾用用户输入关键词/正则将诉求路由到固定 Plan/Exec 链路，导致多轮迭代后架构回退、绕过 LLM 编排。

**禁止行为**：

- 在 `plan/`、`agent/agent/` 中根据用户**原文**匹配寒暄/算术/工具 cue 等直接选步或选 Agent  
- 重新引入 `Classify`、`TierTrivial`、`LightStep`、`reGreeting` 等标识  

**必须行为**：

- Plan 分流由 `PlanAgent` LLM JSON + 代码状态机完成  
- Tier 缺省可由 `inferStepTier` 根据**步骤 instruction**（Plan 生成文本）推断，而非用户 query 正则  

**执行方式**：

- `go test ./lintcheck/...` → `TestNoUserInputRuleRoutingInPlanPath`  

---

## GR-002 — Plan 单步必须结构化回执

**事故来源**：Exec 仅返回自然语言、无 `report_step_result` 时，Plan 无法验收，易误判完成或丢失 artifacts/tools 审计链。

**禁止行为**：

- Plan 步以「模型口头完成」作为 `StepCompleted`  
- 跳过 `report_step_result` 仍标 ok  

**必须行为**：

- 每步结束调用 `report_step_result`（`status` + `summary` + 可选 `artifacts` / `tools_called`）  
- `status=ok` 必经 `verify.Gate`（**当前仅 Layer 1 验盘生效**；Layer 2 见 `gate.go` `layer2AuditEnabled`）  

**执行方式**：

- `classifyStepOutcome` 逻辑审查  
- `plan/verify/gate_test.go`  
- 建议：集成测试断言无 report 时必 fail  

---

## GR-003 — Plan 步 Exec 不得跨步保留 ChatHistory

**事故来源**：Behavior 多轮 tool 结果写入 `ChatHistory` 后，下一步仍可见上一步 MCP 输出，导致串话、重复劳动或错误路标。

**禁止行为**：

- 在 `IsPlanStepExecution` 为 true 时保留上一步 assistant/tool 消息  

**必须行为**：

- 每步 `Process` 开始前 `ChatHistory.Clear`（及 `ClearStepReport`）  
- 跨步上下文仅通过 `buildStepCommand` + `FormatRoadmapForExec` 注入  

**执行方式**：

- 代码审查 `behaviorAgent.go`  
- 建议：架构测试 mock Memory 验证连续两步 Clear  

---

## GR-004 — Exec 能力二层披露，禁止一次塞满 Schema

**事故来源**：system prompt 塞入全部 MCP Schema 导致上下文爆炸、长迭代后约束丢失（与 OpenAI Harness「地图非百科全书」一致）。

**禁止行为**：

- 在 PlanAgent 或 Exec 第一层 system 中枚举全部工具 JSON Schema  
- 移除 `get_capability_details` 而改为一次性列出所有工具  

**必须行为**：

- 第一层：`FormatCatalogForExecutor`（目录）  
- 第二层：按需 `get_capability_details` 解锁工具  

**执行方式**：

- `capabilities/tool_disclosure_test.go`、`agent_catalog_test.go`  
- 仓库根 `AGENTS.md` 运行时约定  

---

## GR-005 — 禁止无记忆命中走 Exec-Simple（阶段二）

**事故来源**：（预防）若仅因「想快」走 simple，会复制错误路径并绕过逐步 Gate，导致架构回退。

**禁止行为**：

- 在 Memory MCP retrieve **未命中**或相似度低于冻结阈值时进入 Exec-Simple  
- 用对话摘要或 SKILL 全文替代结构化路径命中  

**必须行为**：

- Plan 记录 retrieve 结果与路由理由（写入 TodoList Feedback 或等价日志）  
- 未命中 → 阶段一 TodoList + Exec  

**执行方式**：

- 实现后：`ACCEPTANCE_RULES.md` I1/I2 集成测试  
- Code review Plan 路由分支  

---

## GR-006 — Simple 失败必须降级 Exec（阶段二）

**事故来源**：（预防）Simple 失败后原地重试 simple 会导致错误路径固化、无法逐步验盘。

**禁止行为**：

- Simple episode 失败后再次下发同一 TodoList-simple 而不刷新  
- 跳过 pitfall store 直接 adjust simple  

**必须行为**：

- `memory store` 失败摘要 / pitfall  
- Plan 生成**新**保守 TodoList，走逐步 Exec（F2-6）  

**执行方式**：

- 实现后：`ACCEPTANCE_RULES.md` I6 集成测试  

---

## 待沉淀（模板）

```markdown
## GR-XXX — [标题]

事故来源：
禁止行为：
必须行为：
执行方式：
```
