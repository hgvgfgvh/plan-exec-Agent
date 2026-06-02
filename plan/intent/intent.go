// Package intent 仅保留 Plan 编排的数值常量。
//
// 禁止在本包（及 Plan 路径）根据用户输入原文做关键词/正则匹配后绕过 LLM 直接进入固定步骤模板。
// 拆步、寒暄/复杂判定均由 PlanAgent 调用模型动态决策。
package intent

// DefaultPromptMaxSteps 为 PlanAgent prompt 的默认步骤上限提示。
// 实际值由 config/executor.plan_prompt_max_steps 覆盖（读取逻辑在 plan agent 中实现）。
const DefaultPromptMaxSteps = 12
