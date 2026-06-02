package runcontrol

import (
	"context"
	"fmt"
	"strings"
)

// TryAutoSubmitSimpleEpisodeFinal 在 Exec-Simple episode 轮回中，模型以纯文本结束且尚未提交
// report_step_result 时，用最终正文补写 episode 级回执。调用方须已确认：本轮无工具调用，
// 且上一轮执行的工具不是 report_step_result。
func TryAutoSubmitSimpleEpisodeFinal(ctx context.Context, answer string) (StepReport, bool) {
	if !IsPlanSimpleExecution(ctx) {
		return StepReport{}, false
	}
	turnID, _ := TurnMetaFromContext(ctx)
	if turnID == "" {
		return StepReport{}, false
	}
	if rep, ok := PeekStepReport(turnID); ok && strings.TrimSpace(rep.Status) != "" {
		return StepReport{}, false
	}
	summary := strings.TrimSpace(answer)
	if summary == "" {
		return StepReport{}, false
	}
	SetStepReport(turnID, StepReport{
		Status:      "ok",
		Summary:     summary,
		UserVisible: summary,
	})
	fmt.Printf("\n[executor] Exec-Simple 已兜底补写 report_step_result（最终正文）\n")
	rep, ok := PeekStepReport(turnID)
	return rep, ok
}
