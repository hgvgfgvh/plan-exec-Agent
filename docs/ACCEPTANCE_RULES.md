# 验收规则（Plan 用户交付）

## 基石：Plan 门户正文

| 场景 | 预期 |
|------|------|
| 单步「列 MCP/SKILL」且 `summary` 含结构化列表 | 门户正文须包含列表要点，**不得**仅为「我来执行本步…」类过程句 |
| `UserVisible` 为过程句、`summary` 为交付 | `ResolveStepDisplay` 须返回 `summary` |
| 多步任务完成 | `buildUserFacingReply` 调用交付助手或拼接各步 `ResultDetail` |
| 编排进行中 | Behavior **不得**向门户 `PublishFacadeDedup`（`IsPlanStepExecution`） |

## 不变量（回归时禁止破坏）

- `get_capability_details` 仍负责第二层 Schema；**不**因 Plan 交付层而禁用渐进披露。
- `report_step_result` 仍为 Plan 单步验收必填。
- TodoList 持久化路径仍为 `WorkSpace/ToDoList/`。

## Plan 编排上限（config/app.yaml `executor`）

| 配置项 | 默认 | 含义 |
|--------|------|------|
| `plan_prompt_max_steps` | 12 | 拆步 prompt 中的步骤数提示 |
| `plan_max_steps_per_plan` | 24 | 单份 TodoList 硬上限 |
| `plan_max_dispatch_per_turn` | 40 | 单轮用户消息最多下发次数（含重试） |
| `plan_max_adjust_per_step` | 3 | 单步失败后 escalate 阈值 |
| `plan_result_summary_max_runes` | 2000 | 单步 `result_summary` / 失败信息截断 |
| `plan_step_detail_max_runes` | 24000 | 单步 `result_detail`（UserVisible）截断 |

## Tier 与 Gate

| 场景 | 预期 |
|------|------|
| instruction 含「勿再调用」「基于上一步归纳」等 | `ResolveTier` → tier=1；`tools_called: []` 仍通过 Gate |
| tier=2 且需真实调用 MCP/技能 | 无 `tools_called`（除 report）→ Gate 失败 |
| 模型 JSON 写 tier=2 但 instruction 为纯归纳 | 落盘时降为 tier=1，TodoList feedback 记录 `tier N→1` |
