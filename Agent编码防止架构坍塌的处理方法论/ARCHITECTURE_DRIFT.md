# 架构漂移登记

对比 `DESIGN_INTENT.md`（意图）与 `ARCHITECTURE.md`（当前实现）。  
Agent **不得**将下表「违反设计」或「待决策」项自动写入 `ARCHITECTURE.md` 合法化。

分类说明：

| 分类 | 含义 |
|------|------|
| 符合意图 | 实现与宪法一致 |
| 合理演进 | 有意与宪法不同且可接受 |
| 技术债 | 已知缺口，计划补 |
| 需要人工决策 | 是否改代码或改宪法 |
| 违反设计 | 与宪法冲突，应修复或修订宪法 |

---

## 登记项

### DI-1 五层上下文 — 符合意图（部分）

| 项 | 状态 | 说明 |
|----|------|------|
| Plan/Exec 上下文侧重不同 | **符合意图** | 实现符合 DESIGN_INTENT §1–3 |
| 五层无显式类型 | **技术债** | 仅靠 prompt 组装，无结构测试证明五层完备 |

---

### DI-2 TodoList 跨回合恢复 — 需要人工决策

| 项 | 状态 | 说明 |
|----|------|------|
| `todolist.Load` 存在但 `Process` 每次 `NewID` 新建文档 | **需要人工决策** | 设计意图强调 TodoList 为长链路控制台；当前每轮用户诉求一份新 JSON，**不**跨 `Process` 恢复 |

**选项**：A) 保持「一诉求一文件」；B) 支持按 ID/会话恢复未完成计划。

---

### DI-3 Tier 单向上调 — 违反设计（部分）

| 项 | 状态 | 说明 |
|----|------|------|
| Exec 运行中上报更高 tier | **违反设计** | 无 API；tier 仅在计划创建/调节时写入 |
| 模型可填任意 tier 1–3 | **已缓解** | `stepmeta.ResolveTier`：纯归纳 instruction 强制 tier=1，避免与 Gate 冲突 |
| `inferStepTier` 用 instruction 关键词 | **合理演进？** | 非用户原文分流，但与「tier 由 Plan/Exec 上报」意图略有偏差 |

---

### DI-4 Verification Gate 三层 — 技术债 / 违反设计（部分）

| 项 | 状态 | 说明 |
|----|------|------|
| Layer 1 硬规则 | **符合意图** | `artifact.ValidateReportArtifacts`（**当前唯一生效的 Gate 层**） |
| Layer 1 退出码 / linter / 关键字 | **技术债** | 未接 shell 退出码或 golangci-lint；关键字检查未独立实现 |
| Layer 2 行为审计 | **技术债** | 实现于 `auditToolBehavior`，但 `layer2AuditEnabled=false` **临时关闭**；与 DESIGN_INTENT tier≥2 意图不一致，待契约对齐后恢复 |
| L2 artifacts↔write 规则 | **需要人工决策** | 与路标/handoff 语义冲突导致误杀；恢复前需改为「写步须有 artifacts / 读步允许引用」 |
| L2 工具名启发式 | **技术债** | `hasWriteLikeTool` 子串匹配无法覆盖多样 func |
| Layer 3 LLM judge | **违反设计** | **未实现** |
| Tier 3 强于 Tier 2 | **违反设计** | 代码注释：tier 3 与 tier 2 同档（`gate.go`）；且 L2 关闭时 tier 2/3 与 tier 1 验收盘面相同 |
| Gate 输出分流（replan vs 改进重试） | **技术债** | 统一 `StepFailed` → `adjustPlanAfterFailure` |
| Approval Gate | **违反设计** | **未实现** |

---

### DI-5 Exec 清记忆时机 — 符合意图（措辞差异）

| 项 | 状态 | 说明 |
|----|------|------|
| 每 Step 清记忆 | **符合意图** | 在**步开始** `Clear`，非步结束；下一步仍干净 |
| 步内 ReAct history | **合理演进** | 单步内仍有多轮 tool 对话，属 Exec「重 func 细节」 |

---

### DI-6 Plan 调节不重复能力概览 — 技术债

| 项 | 状态 | 说明 |
|----|------|------|
| `adjustPlanAfterFailure` 仅 `FormatForPrompt` | **技术债** | 初始计划有能力概览，调节路径可能弱化能力边界感知 |

---

### DI-7 配置项 — 技术债

| 项 | 状态 | 说明 |
|----|------|------|
| `executor.plan_step_max_steps` | **技术债** | 默认在 `config.go`，`app.yaml` 未列出 |
| `executor.plan_max_steps` | **技术债** | 配置存在但 Plan 循环未使用该上限 |

---

### DI-8 静态护栏 — 符合意图

| 项 | 状态 | 说明 |
|----|------|------|
| 禁止用户原文规则路由 | **符合意图** | `lintcheck/no_rule_routing_test.go` |

---

## 阶段二（2026-05-20 宪法 · **主路径已实现** · 2026-05-24 同步代码）

### DI-9 外置 Memory MCP — 符合意图（部分）

| 项 | 状态 | 说明 |
|----|------|------|
| Memory MCP 客户端 | **已实现** | `plan/memoryhook/mcp_provider.go`：`RegisterProvider("mcp")`，stdio 连接 `plan_memory_hook.mcp_command`，调用 `memory_retrieve` / `memory_store` |
| OnTurnRetrieve | **已实现** | `portal/gateway.go` → `RetrieveTurnBeforeProcess` → `InjectTurnHints`（回合前跨会话参考） |
| OnTurnStore | **已实现** | `portal/gateway.go` → `StoreTurnAfterProcess`（异步 `memory_store`，含 TodoList 终态；`store_enabled: false` 可关） |
| 路由 retrieve | **已实现** | `DecideRoute` → `Provider.Retrieve` → 解析 hints 中 `---memory-route---` / confidence |
| 结构化 Schema | **技术债** | Host 与 Memory MCP 仍为**字符串 + hints 解析**协议，无冻结 JSON Schema 快照测试 |
| noop 回退 | **已实现** | `provider: noop` 或未启用 hook 时不连 MCP、不伪造 simple |

### DI-10 Exec-Simple — 已实现（TodoList-simple 形态为合理演进）

| 项 | 状态 | 说明 |
|----|------|------|
| Exec-Simple 执行体 | **已实现** | `agent/agent/execSimpleAgent.go`，`capabilities.attach_to` 含 `execSimpleAgent` |
| Plan 路由与下发 | **已实现** | `planAgent.Process` → `DecideRoute` → `runExecSimpleEpisode` / `buildSimpleEpisodeCommand` |
| TodoList-simple | **合理演进** | 复用 `todolist.Document`，`execution_mode: simple`；下发**初始拆步链**而非 Memory 单独压缩 JSON（宪法 §14 允许实现时冻结形态） |
| Episode 级回传 Plan | **已实现** | `classifySimpleOutcome` + `verify.Gate` + `applySimpleSuccess`；episode 内仅一次 `report_step_result` |

### DI-11 路由与降级 — 已实现（pitfall 语义部分）

| 项 | 状态 | 说明 |
|----|------|------|
| 把握度评估（复杂问题禁止 simple） | **已实现** | `exec_simple_max_tier` + `exec_simple_min_confidence` + Memory MCP 返回的 matched/confidence |
| Simple 失败 → 保守 Exec | **已实现** | `runExecSimpleEpisode` 阻塞 simple 文档 → 新建 `-exec` TodoList → `runConservativeExecLoop`（F2-6） |
| Pitfall 写入 Memory MCP | **部分实现** | 失败回合经 `StoreTurnAfterProcess` 写入 episode 内容（含 `ProcessError`、blocked 终态）；**无** simple 失败时独立的 `kind=pitfall` 同步写入 |

### DI-12 Skill 沉淀候补 — 技术债

| 项 | 状态 | 说明 |
|----|------|------|
| 成功 episode → skill pack 草稿 | **技术债** | 有 `skill_packs` 机制，无与 Memory 闭环联动 |

### DI-13 阶段二速度诉求 vs 现网 — 需要人工决策

| 项 | 状态 | 说明 |
|----|------|------|
| 逐步下发慢 | **符合阶段一** | 阶段一设计取舍；阶段二旨在缓解，非修 bug |

**实现顺序建议（人工）**：~~Memory MCP 契约 → Plan 路由 → TodoList-simple → Exec-Simple → 降级 + store~~（**主路径已完成**）→ pitfall 语义对齐 → Memory↔Simple 集成测试 → Skill 候补。

---

### DI-14 阶段三 Soul MCP — 技术债（规划）

| 项 | 状态 | 说明 |
|----|------|------|
| `plan_soul_hook` + `soulhook` 包 | **技术债** | 宪法 F3-1～F3-9 已确立；Host 无 retrieve/store |
| Soul MCP 进程 | **技术债** | `AgentTestSoulMCP` 仅文档，无 `soul-mcp` 可执行文件 |
| 注入顺序 soul→memory | **技术债** | `portal/gateway` 仅 Memory hook |
| `soul.config` 基座 | **技术债** | 规划在 Soul 仓库根；未与 Host `paths.soul`（Nexus.yml）对齐决策 |

**实现顺序建议（人工）**：Soul MCP 契约冻结 → `soul-mcp` MVP（模板 retrieve）→ `plan_soul_hook` → 与 Memory 并行集成测试 → 可选 LLM compose。

---

### DI-15 Soul vs Memory 边界 — 符合意图（规划）

| 项 | 状态 | 说明 |
|----|------|------|
| 人格/议题不入 Memory factworld | **符合意图** | 阶段三宪法 F3-1、F3-2 |
| Soul 不参与 simple 路由 | **符合意图** | F3-7；待实现时须防 hints 污染 `---memory-route---` |

---

### DI-16 Soul vs sessionmemory — 需要人工决策

| 项 | 状态 | 说明 |
|----|------|------|
| 近轮原话 vs 跨会话议题 | **需要人工决策** | `sessionmemory` 保留；Soul 是否替代部分 `plan_memory` JSONL 用途待冻结 |
| Affective `Nexus.yml` vs Soul retrieve | **需要人工决策** | 双源人格是否合并、谁优先 |

---

## 优先级建议（人工）

**阶段一（现网）**

1. **P0**：**L2 契约对齐并恢复**（`layer2AuditEnabled`）— 先于 Tier 3 / L3 judge  
2. **P0**：Tier 3 / L3 judge / Approval — 若坚持宪法条文  
3. **P1**：Tier 只升不降（Exec 上报调高）  
4. **P2**：TodoList 跨回合是否恢复  
5. **P3**：Gate 失败按 Layer 分流  

**阶段二（主路径已落地，剩余优化）**

6. ~~**P0**：Memory MCP retrieve/store + Plan 路由~~ → **已完成**（见 DI-9～DI-11）  
7. **P1**：Memory MCP 与 Exec-Simple **端到端集成测试**（真实 `memory-mcp.exe`，非 stub）  
8. **P2**：Simple 失败 **显式 pitfall store**（`kind=pitfall` 或 Memory MCP 等价 API）  
9. **P3**：Skill 沉淀候补流水线；可选「Memory 压缩 TodoList-simple」形态  

**阶段三（规划，代码未落地）**

10. **P1**：Soul MCP 契约 + `soul.config` 基座 + 模板 retrieve MVP  
11. **P2**：`plan_soul_hook` + portal 注入顺序（soul → memory）  
12. **P3**：可选 LLM compose / `soul_overlay` 审计 UI  

---

## 变更记录

| 日期 | 变更 |
|------|------|
| 2026-05-19 | 初版：对照代码库梳理 8 项设计意图 |
| 2026-05-19 | 同步代码：`layer2AuditEnabled=false`，L2 临时关闭；DI-4 / ACCEPTANCE_RULES E 节更新 |
| 2026-05-20 | 阶段二宪法入 `DESIGN_INTENT`；DI-9～DI-13；`ARCHITECTURE.md` §14；`ACCEPTANCE_RULES.md` §I |
| 2026-05-24 | 对照代码：Memory MCP Provider + Exec-Simple 主路径已实现；DI-9～DI-11、README、ARCHITECTURE §14、ACCEPTANCE §I 同步 |
| 2026-05-24 | 阶段三 Soul MCP 宪法入 `DESIGN_INTENT` §阶段三；`ARCHITECTURE` §15；DI-14～DI-16；`ACCEPTANCE` §J；`AgentTestSoulMCP/docs` 初版 |
