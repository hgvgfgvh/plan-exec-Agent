package runcontrol

import (
	"context"
	"strings"
)

// AutoSubmitAfterSkillResult 在 Plan 单步且已有真实技能输出时，自动写入 report_step_result，
// 避免模型在下一轮只输出自然语言却未调用 report_step_result 导致 Plan 判失败。
func AutoSubmitAfterSkillResult(ctx context.Context, skillSummary string, toolsCalled []string) bool {
	turnID, _ := TurnMetaFromContext(ctx)
	if turnID == "" {
		return false
	}
	summary := strings.TrimSpace(skillSummary)
	if summary == "" {
		return false
	}
	if toolsCalled == nil {
		toolsCalled = []string{}
	}
	SetStepReport(turnID, StepReport{
		Status:      "ok",
		Summary:     summary,
		ToolsCalled: toolsCalled,
	})
	return true
}
