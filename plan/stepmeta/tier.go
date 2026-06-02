// Package stepmeta 提供 Plan 步骤元数据规则（tier、纯文本步判定），供 PlanAgent 与验收闸门对齐。
package stepmeta

import "strings"

// AllowsNoToolExecution 步骤 instruction 是否要求本步不调用 MCP/内置技能（纯文本、归纳、寒暄等）。
func AllowsNoToolExecution(instruction string) bool {
	b := strings.ToLower(strings.TrimSpace(instruction))
	if b == "" {
		return false
	}
	for _, m := range []string{
		"无需调用", "无需使用", "不需调用", "不要调用", "勿调", "勿再调", "勿再调用",
		"无需任何工具", "无需工具", "不调用任何", "不调用其它", "不调用其他",
		"无需 mcp", "无需技能", "直接归纳", "直接总结", "仅归纳", "仅总结",
		"基于上一步", "基于步骤", "基于已完成", "不要调用list", "勿调用",
		"without tool", "no tools", "do not call any tool",
	} {
		if strings.Contains(b, m) {
			return true
		}
	}
	return false
}

// InferTier 根据步骤内容推断验收等级（1 轻 / 2 标准 / 3 重）。
func InferTier(title, instruction string, hints []string) int {
	if len(hints) == 0 && AllowsNoToolExecution(instruction) {
		return 1
	}
	blob := strings.ToLower(title + " " + instruction)
	for _, kw := range []string{"重构", "迁移", "安全", "支付", "删除", "refactor", "migration", "security", "payment"} {
		if strings.Contains(blob, kw) {
			return 3
		}
	}
	if len(hints) > 3 || len([]rune(instruction)) > 280 {
		return 2
	}
	return 2
}

// ResolveTier 合并模型给出的 tier 与架构规则：纯文本/归纳步强制 tier=1，避免与 verify.Gate 工具审计冲突。
func ResolveTier(modelTier int, title, instruction string, hints []string) int {
	if AllowsNoToolExecution(instruction) {
		return 1
	}
	if modelTier >= 1 && modelTier <= 3 {
		return modelTier
	}
	return InferTier(title, instruction, hints)
}
