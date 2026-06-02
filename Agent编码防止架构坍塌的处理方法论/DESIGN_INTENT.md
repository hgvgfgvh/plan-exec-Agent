# 设计意图（宪法）

> **维护规则**：人工维护；Agent 不得用「实现更方便」覆盖下文取舍。新决策以日期段落**追加**，不删除历史。  
> **阶段关系**：阶段二 **扩展** 阶段一；阶段三 **扩展** 阶段一/二，**不替代** 任一阶段。冲突时以对应阶段铁律与决策日志为准。  
> **外置仓库**：执行经验 → `AgentTestMemoryMCP`；人格/议题/协作适配 → `AgentTestSoulMCP`（阶段三，**规划/未实现**）。

---

## 阶段二铁律（宪法级 · 2026-05-20 确立）

以下在实现阶段二时 **不可破坏**；编码 Agent 不得用「实现更快」绕过。

| # | 铁律 | 含义 |
|---|------|------|
| F2-1 | **阶段一保底** | `plan → exec`（逐步 TodoList + `report_step_result` + Gate）在 simple 不可用、失败降级、高 tier/无把握时必须可用 |
| F2-2 | **Exec 兜底** | BehaviorAgent/Exec 是**保守执行路径**；复杂、高风险、无记忆命中、simple 失败后的唯一主路径 |
| F2-3 | **Memory MCP 权威** | 结构化经验、成功路径、pitfall 优化知识由**外置 Memory MCP** 读写；不得仅用对话摘要冒充「可复用路径」 |
| F2-4 | **Plan 裁决路由** | 是否走 simple、是否一次吞并多步、是否降级 exec，由 **Plan + 记忆检索结果 + 把握度评估** 决定，非 Exec-Simple 自决 |
| F2-5 | **Episode 级回传** | Exec-Simple 运行期间 **不向 Plan 逐步汇报**；仅在 **成功完成 episode** 或 **路径级报错** 时结构化回传 Plan |
| F2-6 | **失败必降级** | Simple 失败 → 写入记忆（pitfall/失败摘要）→ Plan 生成**新**保守 TodoList → 走 Exec 逐步路径；禁止在同一 simple episode 内无限硬扛 |
| F2-7 | **验收合流** | Simple 成功回传 Plan 时仍须带 **可验收载荷**（摘要、artifacts、工具迹）；Plan 推进前可按 tier 走 Gate 或等价检查 |
| F2-8 | **Skill 候补** | Skill 沉淀是**候补**优化手段；主路径是 Memory MCP + Exec-Simple，不得用「再写一个 SKILL」替代记忆闭环 |
| F2-9 | **速度不牺牲边界** | 经验加速不得破坏阶段一的职责分离（Plan 无工具、Exec 步间清记忆、防用户原文规则分流） |

---

## 阶段三铁律（宪法级 · 2026-05-24 确立 · 规划未实现）

以下在实现 **Soul MCP（人格/用户上下文 MCP）** 时 **不可破坏**；与阶段二 **并行**，不得用 Soul 能力冒充 Memory 经验或替代阶段一保底。

| # | 铁律 | 含义 |
|---|------|------|
| F3-1 | **体验轨独立** | **人格画像、议题续接、协作口吻** 由 **外置 Soul MCP** + Host `plan_soul_hook` 负责；**不得**塞入 Memory MCP 主库 |
| F3-2 | **Memory 边界不变** | Memory MCP **仅**执行经验（工具链、pitfall、Exec-Simple 路由）；F2-3～F2-9 仍有效 |
| F3-3 | **双工具契约** | Soul MCP 对外 **仅** `soul_store` / `soul_retrieve`（名称实现前可冻结）；字符串 JSON；**禁止** Host 可见细粒度画像 API |
| F3-4 | **存慢取快** | `soul_store` 同步 ACK + 异步整理（可含 LLM）；`soul_retrieve` 同步、有预算；失败 **不阻断** 主流程 |
| F3-5 | **WebUI 对话为源** | Store 材料 = 用户与主 Agent 在 **WebUI** 的交互历史（Host 序列化后传入）；**禁止** Behavior 步内 ReAct 工具观测冒充「用户对话」 |
| F3-6 | **注入顺序** | 回合开始：`soul_retrieve`（人格+议题）→ `memory_retrieve`（执行经验）→ 用户本轮输入；**禁止** 颠倒导致路由/指代错乱 |
| F3-7 | **不参与 Exec 路由** | Soul retrieve **不得**输出 `exec_simple_match` 或等价快路径裁决；simple 仍 **仅** Memory hook |
| F3-8 | **soul.config 基座** | 仓库旁 `soul.config` 为 Agent **人格基座**；MCP **默认只读**；LLM 改写须落 `soul_overlay` 且可审计，**禁止**静默覆盖基座文件 |
| F3-9 | **进化前期投入** | 本阶段目标是 **舒适度/一点就通/冷启动消减**，非自主改代码或改 Host 宪法；「自我进化」须另立决策与验收 |

---

## 阶段三架构 — 核心意图（2026-05-24 确立 · 规划未实现）

### 17. 目标（心心相印 / 使用体感）

| 用户痛点 | 阶段三意图 |
|----------|------------|
| 昨日聊过的项目/论文，今日不想重复铺垫 | **历史事件/议题配置** 跨会话 retrieve，支撑指代消解 |
| 口头禅、称呼、回复长度与风格 | **用户人格画像**（可验证条目）注入 Plan/Affective 参考 |
| 进入系统「冷启动」感强 | 首轮 retrieve 即带 **协作上下文 + persona**，减少用户重复引导 |
| 执行仍要快、要稳 | **不**替代 Memory；阶段一/二保底不变 |

定位为 **自我进化体系的前期投入**：先形成 **可审计、可钩子化、可独立演进** 的「人—事—风格」记忆层，而非在 Host 内隐式堆 prompt。

### 18. Soul MCP（外置 · 与 Memory MCP 同级）

| 职责 | 说明 |
|------|------|
| **存入** | Host 在回合结束（及可选增量）调用 `soul_store`：传入 WebUI **对话 episode** 字符串；MCP **异步** 抽取/整理为 `profile` + `events` + 可选 `soul_overlay` |
| **取出** | 用户新消息前 Host 调用 `soul_retrieve`：传入 **当前轮** `context`（含用户输入 + 可选本轮迄今对话摘录）；返回 **人格提示词块** + **历史事件/议题配置块** |
| **soul.config** | 与 MCP 工程同级的 **基座人格** YAML；retrieve 时合并进 `persona_prompt`；动态维护仅限 overlay，见 F3-8 |

**边界**：Soul MCP **不负责**工具链成功经验、pitfall 抑制、TodoList 编排；**不负责** Plan 拆步。

子系统宪法与架构见独立仓库：`C:\DATA\GODATA\AgentTestSoulMCP\docs\`（`DESIGN_INTENT.md`、`ARCHITECTURE.md` 等）。

### 19. Host 钩子（主项目内 · 对称 memoryhook）

| 钩子 | 时机 | 行为 |
|------|------|------|
| `OnTurnRetrieve` | 门户 `RunRouterTurn`、Plan 处理前 | stdio 调 `soul_retrieve` → 拼入 `planInput`（在 Memory hints **之前**） |
| `OnTurnStore` | 回合结束 | 异步 `soul_store`（整轮 WebUI 对话序列化） |
| **无** `DecideRoute` | — | Soul **不参与** Exec-Simple 路由（F3-7） |

建议配置段：`plan_soul_hook`（`app.yaml`），形态对齐 `plan_memory_hook`（`enabled`、`mcp_command`、`mcp_env`）。

### 20. 记忆分层（阶段三视角）

```text
┌─────────────────────────────────────────────────────────────┐
│ 第一层 Host：sessionmemory + plan_memory.jsonl（会话/近轮）    │
├─────────────────────────────────────────────────────────────┤
│ Soul MCP（第二层·体验）：profile + events + soul.config      │
├─────────────────────────────────────────────────────────────┤
│ Memory MCP（第三层·执行）：facts + 图 + Exec 路由            │
└─────────────────────────────────────────────────────────────┘
```

- **第一层**：仍管「最近几轮原话」与 Archiver 压缩。  
- **Soul MCP**：管 **跨会话「在聊什么、用户是谁」**。  
- **Memory MCP**：管 **怎么干活、曾失败什么工具路径**。

三层 **并行**；Host 负责 **dedupe**（避免同一原文三次 RAG）与 token 预算。

### 21. 与 Affective / Plan 的人格分工

| 组件 | 人格来源（规划） |
|------|------------------|
| **PlanAgent** | `soul_retrieve.persona_prompt` + `event_context`（编排/指代） |
| **AffectiveInteractiveAgent** | 可继续 `Nexus.yml`；或配置为 **同样消费** Soul retrieve（待实现前冻结） |
| **BehaviorAgent** | **不**直接消费 Soul MCP（防执行分心） |

### 22. 明确不做（阶段三）

- 不把人格/议题写入 Memory `facts.jsonl` 作为主路径。  
- 不做 Soul MCP 多工具暴露、不做 Host 侧画像编辑 API（v1）。  
- 不做 retrieve 默认重 LLM（须像 Memory 一样可配置、可超时回退）。  
- 不以 Soul 沉淀 SKILL 或替代 `skill_packs`。

---

## 阶段二架构 — 核心意图（2026-05-20 确立）

在阶段一 `plan → exec` 之上扩展为：

```text
plan ── (结构化 Memory MCP) ── [ exec & exec-simple ] + skill沉淀(候补)
```

### 10. 为何要 Exec-Simple（问题陈述）

阶段一：Plan 生成 TodoList → **每一步** 下发 Exec → 每步 `report_step_result` → Plan 再发下一步。  
优点：边界清晰、可验收、抗坍缩。  
缺点：**即使用户需求与历史成功案例高度相似，仍逐步调度**，无法利用已验证路径提速——属于**架构性限速**，非模型慢 alone。

阶段二目标：**用得越久、成功路径越多，重复类任务应更快**，且失败可回退到阶段一保守路径。

### 11. Memory MCP（外置结构化记忆）

| 职责 | 说明 |
|------|------|
| 结构化记忆 | 任务类型、步骤序列、工具链、产物路径、约束条件等**可检索**结构 |
| 优化经验体系 | 成功 episode 沉淀、失败 pitfall、可选评分/版本 |
| 与 Plan 的关系 | Plan 在拆步/路由前 **retrieve**；调节与降级时 **store** 失败与成功摘要 |

**边界**：Memory MCP 不负责替代 Plan 编排；不挂载到 Plan 作为「执行工具」，而是 **Host/Plan 通过 MCP 客户端** 做 retrieve/store（具体 API 在实现阶段冻结于 Memory MCP 仓库宪法）。

### 12. Exec-Simple（快路径执行体）

| 维度 | Exec（阶段一） | Exec-Simple（阶段二） |
|------|----------------|----------------------|
| 触发 | Plan 默认逐步下发 | Plan 判定：记忆命中 + 非「无把握的一次性复杂问题」 |
| 输入 | 单步 `buildStepCommand` | **TodoList-simple**（合并历史成功路径 + 本轮需求差分） |
| 运行方式 | 一步一清记忆、一步一 report | **episode 内持续执行**（多步/多工具可在同一执行上下文完成） |
| 对 Plan 反馈 | 每步 | **仅成功或路径级失败** |
| 风险 | 慢但稳 | 快但必须有降级与记忆闭环 |

### 13. Plan 路由逻辑（设计意图）

在 **Plan 阶段**（生成或调节计划时）：

1. 通过 Memory MCP **检索**是否已有实现成功的经验路径（相似任务/步骤序列/产物模式）。  
2. **评估把握度**：若属于「没把握一次处理通的复杂问题」（高 tier、跨模块、安全、强不确定性）→ **禁止** simple，走阶段一 TodoList + Exec。  
3. 若命中且把握度足够：  
   - **提取**历史成功路径（步骤骨架、工具链提示、产物约定）；  
   - 与**本轮新需求**合并、总结合成 **TodoList-simple**；  
   - 下发给 **Exec-Simple** 持续执行。  
4. **仅当** Exec-Simple **成功** 或 **路径报错** 时回传 Plan：  
   - **成功** → Plan 标记 episode 完成，可选 store 强化、推进后续或结束；  
   - **失败** → Plan 整理**新**保守 TodoList，走 **Exec** 逐步路径（F2-6）。

```text
                    ┌─ retrieve 命中 + 把握度 OK ──► TodoList-simple ──► Exec-Simple
用户诉求 ──► Plan ──┤                                                      │
                    │                              成功 / 路径失败 ◄──────┘
                    └─ 未命中 / 复杂 / simple 失败 ──► TodoList ──► Exec（逐步）
```

### 14. TodoList-simple（快路径控制台）

- 形态：可与 TodoList 同目录不同后缀或 `execution_mode: simple` 字段（实现时冻结）。  
- 内容：基于记忆路径的**压缩步骤链** + 本轮需求差分，**非**阶段一逐条细拆的 24 步满表。  
- 生命周期：绑定单次 simple episode；失败后不原地修补 simple，由 Plan **新开**保守 TodoList。

### 15. Skill 沉淀（候补）

- 当 Memory MCP 无法表达的路径（极少见）或需人工可读 playbook 时，可沉淀为外挂 SKILL。  
- **不得**替代 Memory MCP 的主优化闭环（F2-8）。  
- 与 `WorkSpace/skill_packs` 机制兼容，由 Plan/Host 显式引用。

### 16. 三类数据链路（与 CLCA 对齐）

| 链路 | 阶段二要点 |
|------|-----------|
| **业务流** | 用户诉求 → Plan →（simple 或 exec）→ 用户交付 |
| **优化流** | episode 成功 → Memory MCP store；下次 retrieve → Exec-Simple |
| **反馈流** | simple 失败 → pitfall store → Plan 降级 Exec；Gate/验收失败 → 同阶段一 |

---

## 阶段一架构 — 核心意图（2026-05-19 确立）

### 1. 上下文分 5 层（背景 · 能力 · 状态 · 反馈 · 需求）

PlanAgent 与 ExecAgent（BehaviorAgent）**各有侧重**，不是同一套 prompt 堆叠。

| 层 | PlanAgent 侧重 | ExecAgent 侧重 |
|----|----------------|----------------|
| 背景 | 跨轮记忆、本能、经验、对话压缩 | 本步指令内的灵魂/本能；**不**依赖跨步 ChatHistory |
| 能力 | 能力体系**名称级**概览 | 完整工具表 + `get_capability_details` 二层披露 |
| 状态 | TodoList 全文（调节时） | 单步 descriptor + 已完成路标 |
| 反馈 | TodoList `Feedback` 链 | 步内 ReAct 工具观测 |
| 需求 | `UserRequirement` + 指代解析 | `用户总需求` + `本步任务` |

### 2. PlanAgent：重逻辑链路，弱化能力细节

- 多轮对话历史记忆（`plan_memory` JSONL + buffer + archiver）  
- 不挂载 MCP/技能执行工具；编排由**代码驱动**循环，非 ReAct 拆步  
- 能力仅 `BuildPlanCapabilityOverview()` 名称级引用  

### 3. ExecAgent：重能力细节，弱化逻辑链路

- Plan 单步内 ReAct + 工具 Schema + `SetExecutorStep` / MCP  
- **每步开始清空**行为脑区 `ChatHistory`，避免跨任务串话  
- 不负责维护整份 TodoList 逻辑（只回报 `report_step_result`）  

### 4. PlanAgent 维护落地 TodoList 文件

路径：`WorkSpace/ToDoList/{planId}.json`  

作为长逻辑链路的**控制台**：

- 规划基于它（初始 JSON 计划写入）  
- 报错/调度/结果/调节/升级 写入 `Feedback`  
- 失败调节与人工排查可参考同一文件  

### 5. ExecAgent 完成一个 Step 就清记忆

每步下发前清空 Behavior `ChatHistory`（及本回合 `StepReport`），下一步靠 descriptor + 路标重建上下文。

### 6. 路标注入（Step descriptor）

Plan 下发新 Step 时，将 TodoList 中**已完成/已跳过**步骤的摘要、产出路径、已用工具写入 `【已完成步骤路标】`，随 `buildStepCommand` 带给 Exec。

### 7. TodoList 双来源更新

| 来源 | 说明 |
|------|------|
| a. 模型 | Plan 初始/调节 JSON；Exec 不直接改 TodoList 文件 |
| b. 工具/代码埋点 | `report_step_result`、`RecordPlanToolCall`、`AppendFeedback`、状态机、`RecordStepOutcome` |

### 8. Step 复杂度 Tier（1 / 2 / 3）

| Tier | 含义 | 典型场景 |
|------|------|----------|
| 1 | 轻 / 低风险 | 寒暄、纯文本、instruction 写明勿调工具 |
| 2 | 标准 | 常规 MCP/技能变更 |
| 3 | 重 / 高风险 | 重构、安全、支付、跨模块等 |

**意图**：Exec 发现实际更复杂时可**单向上调** tier（1→2→3），不可下调。  
Tier 决定 Verification Gate 力度。

### 9. Verification Gate（Exec → Plan 必经）

按 tier 触发分层检查；Gate 结论驱动 Plan：推进 / 带错重试 / replan / 品质重试 / 终止。

| Tier | 设计意图 |
|------|----------|
| 1 | Layer 1 基础硬规则（退出码语义、产物存在等） |
| 2 | Layer 1 完整 + Layer 2 行为审计（linter 类、关键字、tool_calls 一致性） |
| 3 | Layer 1 + Layer 2 + Layer 3 LLM judge +（可选）人工 approval |

> **实现偏差（临时）**：`plan/verify/gate.go` 设 `layer2AuditEnabled=false`，tier≥2 的 **Layer 2 未执行**，仅 L1 验盘。宪法上 tier 2/3 仍含 L2；恢复前见 `ARCHITECTURE.md` §9、`ARCHITECTURE_DRIFT.md` DI-4。

**Gate 输出语义（设计）**：

- 真通过 → 下一 Step  
- 客观失败 → Exec 带错误重试  
- 行为审计失败 → 撒谎/失控 → replan  
- 品质不达标 → Exec 改进重试  
- 批准被拒 → 终止或换方案  

---

## 决策日志

### 2026-05-19 — 采纳 Harness 文档体系

**设计意图**：将阶段一 Plan/Exec 架构固化为仓库内可版本化的「宪法 + 架构图 + 验收规则」，对抗 Agent 长迭代坍缩。  

**原因**：对话上下文会截断；仅靠 `AGENTS.md` 无法约束编排边界。  

**影响**：

- 编码前必须读 `DESIGN_INTENT.md` 与本目录 `ARCHITECTURE.md`  
- 不变量优先写入 `go test` / `lintcheck`，而非加长 system prompt  
- `AGENTS.md` 保持「能力地图」，不混入编排宪法  

### 2026-05-19 — L2 行为审计临时关闭（实现层，未改宪法）

**背景**：L2 规则（`tools_called` 非空、artifacts↔write 子串、死循环计数）与多样 MCP/只读 handoff 步冲突，Plan 误杀 → 调节循环。  

**实现**：`layer2AuditEnabled=false`，`auditToolBehavior` 保留待恢复。  

**影响**：验收以 L1（声明路径则验盘）为主；E2/E3/E5 在 `ACCEPTANCE_RULES.md` 标为暂停。恢复 L2 前须对齐 artifacts 语义（写步 / 读步）。  

### 2026-05-20 — 阶段二：Memory MCP + Exec-Simple + Skill 候补

**设计意图**：在阶段一 `plan-exec` 上增加 **外置结构化 Memory MCP** 与 **Exec-Simple 快路径**，使重复成功任务随使用时间缩短执行路径；失败一律降级阶段一保守 Exec。  

**原因**：阶段一逐步下发在记忆已有成功案例时仍全速逐步调度，属于架构限速；需在保持 Plan/Exec 边界与验收前提下引入 episode 级快路径。  

**影响**：

- 实现须满足上文 **阶段二铁律 F2-1～F2-9**  
- `ARCHITECTURE.md` 增加「阶段二目标」章节；**当前代码仍以阶段一为准**  
- 验收见 `ACCEPTANCE_RULES.md` §I（规划项）；未实现项记入 `ARCHITECTURE_DRIFT.md` DI-9+  
- 与 `.cursor/skills/agent-clca-design-zh` 的优化流/反馈流一致；Memory MCP 子系统宪法见独立 repo（若已存在）  

**开放问题（实现前须人工冻结）**：

1. Memory MCP 的 retrieve/store 契约（输入：用户诉求+上下文 hash；输出：路径模板+pointers）  
2. 「无把握一次性复杂问题」的判定：仅 LLM、仅 tier、或 tier+步骤数+记忆相似度阈值  
3. Exec-Simple 与 BehaviorAgent 是同一 Executor 的 `execution_mode` 还是独立 Agent 注册名  
4. TodoList-simple 与 TodoList 是同文件双视图还是独立 `{id}-simple.json`  
5. Simple episode 成功时 Gate 在 Plan 侧跑一次还是批量 steps 抽样验盘  

### 2026-05-24 — 阶段三：Soul MCP + Host 人格/议题钩（规划）

**设计意图**：在阶段一/二之上，以 **外置 Soul MCP** + **`plan_soul_hook`** 实现人格化记忆、用户特征、议题续接与协作适配（一点就通、减少冷启动），作为 **自我进化体系的前期投入**；与 Memory MCP **进程级隔离**。

**原因**：舒适度与执行经验正交；混在同一 factworld 会污染 Exec 路由与 hints；专家评议倾向 **参考 Memory MCP 钩子模式** 独立演进。

**影响**：

- 满足 **阶段三铁律 F3-1～F3-9**  
- `ARCHITECTURE.md` 增加 §15（目标架构）；**当前代码未实现**  
- 验收规划见 `ACCEPTANCE_RULES.md` §J；未实现项见 `ARCHITECTURE_DRIFT.md` DI-14+  
- Soul 子系统文档：`AgentTestSoulMCP/docs/`（宪法、架构、验收；**无代码**）

**开放问题（实现前须人工冻结）**：

1. `soul_store` / `soul_retrieve` 最终工具名与 JSON 字段  
2. retrieve 默认 **模板组装** vs 可选 LLM compose（建议默认无 LLM）  
3. Affective 是否与 Plan **共用** 同一次 retrieve 结果  
4. `user_id` / 多租户与 WebUI session 绑定  
5. `soul_overlay` 是否需用户确认 UI  
