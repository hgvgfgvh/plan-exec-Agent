# 设计意图（人工维护 · 宪法层）

本文件记录**不可被当前实现自动覆盖**的架构取舍。Agent 同步 `ARCHITECTURE.md` 时须以本文件为准绳。

## 2026-05-19 — Plan 用户交付层（User Delivery）

**设计意图**

- **Plan** 是用户主入口：负责拆步、验收、调节计划，并对**门户最终正文**负责。
- **Behavior（Exec）** 负责真实调用 MCP/技能；单步必须以 `report_step_result` 交卷，但**不得默认独占**用户可见通道。
- **能力体系**（MCP 渐进披露、`get_capability_details`、内置技能、外挂 SKILL）保持独立，不因门户展示策略而改变工具注册与解锁逻辑。

**原因**

- 曾出现「完整结论在 `report.summary`，门户却展示更长过程句 `UserVisible`」的通道错位。
- 用户期望 Plan 作为整体层级归纳反馈，而非 Exec 字段的被动透传。

**影响（Agent/代码须遵守）**

- 门户展示文本须经 `plan/delivery.ResolveStepDisplay(summary, userVisible)` 或等价策略，**禁止**写死「永远优先 UserVisible」。
- 单步任务在素材足够长时，Plan **可以**调用交付助手（`synthesizeUserReply`）做归纳；多步任务继续沿用现有逻辑。
- `progress_to_portal` 为可选进度推送，**不替代**最终 `buildUserFacingReply` 总回复。
- 不得为实现门户修复而修改：MCP 渐进披露、Behavior 工具执行路径、能力目录生成。

## 2026-05-19 — Plan 步骤 tier 与验收闸门对齐

**设计意图**

- `verify.Gate` 在 tier≥2 时要求除 `report_step_result` 外有真实 `tools_called`。
- Plan 常将「获取数据」与「基于上一步归纳呈现」拆成两步；后一步 instruction 写明「勿再调用工具」时，Behavior 正确上报 `tools_called: []`，**不应**因模型误标 tier=2 而失败。

**影响**

- 纯文本/归纳步的 tier 由 `plan/stepmeta.ResolveTier` 在写入 TodoList 时**强制为 1**（依据 instruction 关键词，非用户原文正则分流）。
- 不放宽 tier≥2 的工具审计；不修改 MCP/Exec 执行路径。
