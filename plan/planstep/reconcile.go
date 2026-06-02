package planstep

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/plan/skillwait"
	"context"
	"fmt"
	"strings"
)

// ApplyStepReport 写入步骤回执：忽略占位 fail 覆盖已有 ok；有技能缓存时升格为 ok。
func ApplyStepReport(ctx context.Context, incoming runcontrol.StepReport) {
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	if turnID == "" {
		runcontrol.SetStepReport(turnID, incoming)
		return
	}

	existing, hasExisting := runcontrol.PeekStepReport(turnID)
	incoming = mergeStepReportTools(incoming, turnID)

	if strings.EqualFold(strings.TrimSpace(incoming.Status), "fail") {
		if skillwait.IsPlaceholderSkillSummary(incoming.Summary) {
			if cached, ok := skillwait.PeekCachedResult(turnID); ok {
				incoming = runcontrol.StepReport{
					Status:      "ok",
					Summary:     cached,
					ToolsCalled: incoming.ToolsCalled,
					Artifacts:   incoming.Artifacts,
				}
			} else if hasExisting && strings.EqualFold(existing.Status, "ok") && !skillwait.IsPlaceholderSkillSummary(existing.Summary) {
				return
			}
		}
	}

	if strings.EqualFold(strings.TrimSpace(incoming.Status), "ok") && skillwait.IsPlaceholderSkillSummary(incoming.Summary) {
		if hasExisting && strings.EqualFold(existing.Status, "ok") && !skillwait.IsPlaceholderSkillSummary(existing.Summary) {
			return
		}
		if cached, ok := skillwait.PeekCachedResult(turnID); ok {
			incoming.Summary = cached
		}
	}

	runcontrol.SetStepReport(turnID, incoming)
}

func mergeStepReportTools(incoming runcontrol.StepReport, turnID string) runcontrol.StepReport {
	if len(incoming.ToolsCalled) > 0 {
		return incoming
	}
	// ToolsCalled 由 runcontrol.SetStepReport 从 RecordPlanToolCall 补齐
	_ = turnID
	return incoming
}

// ReconcileSkillStepAfterRun Behavior Run 结束后补齐 SetExecutorStep 异步结果。
func ReconcileSkillStepAfterRun(ctx context.Context, behaviorResult *string) {
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	if turnID == "" || !runcontrol.PlanStepUsedTool(turnID, "SetExecutorStep") {
		return
	}

	if rep, ok := runcontrol.PeekStepReport(turnID); ok && strings.EqualFold(rep.Status, "ok") && !skillwait.IsPlaceholderSkillSummary(rep.Summary) {
		if behaviorResult != nil && strings.TrimSpace(*behaviorResult) == "" {
			*behaviorResult = runcontrol.StepReportUserText(rep)
		}
		return
	}

	if cached, ok := skillwait.PeekCachedResult(turnID); ok {
		applySkillOutput(ctx, behaviorResult, cached)
		return
	}

	needWait := false
	if behaviorResult != nil {
		needWait = skillwait.NeedsWait(*behaviorResult) || skillwait.IsPlaceholderSkillSummary(*behaviorResult)
	}
	if rep, ok := runcontrol.PeekStepReport(turnID); ok {
		if strings.EqualFold(rep.Status, "fail") && skillwait.IsPlaceholderSkillSummary(rep.Summary) {
			needWait = true
		}
	}
	if !needWait {
		return
	}

	fmt.Printf("\n[planstep] Plan 单步：补齐等待 SetExecutorStep 技能结果…\n")
	skillOut, waitErr := skillwait.Wait(ctx, skillwait.DefaultTimeout)
	if waitErr != nil {
		if behaviorResult != nil {
			*behaviorResult = fmt.Sprintf("内置技能执行未完成: %v", waitErr)
		}
		return
	}
	if strings.TrimSpace(skillOut) != "" {
		applySkillOutput(ctx, behaviorResult, skillOut)
	}
}

func applySkillOutput(ctx context.Context, behaviorResult *string, skillOut string) {
	skillOut = strings.TrimSpace(skillOut)
	if skillOut == "" {
		return
	}
	if behaviorResult != nil {
		*behaviorResult = skillOut
	}
	runcontrol.AutoSubmitAfterSkillResult(ctx, skillOut, []string{"SetExecutorStep"})
}
