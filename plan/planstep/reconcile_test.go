package planstep

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/plan/skillwait"
	"context"
	"strings"
	"testing"
)

func TestApplyStepReport_ignorePlaceholderFailOverOk(t *testing.T) {
	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, "t-apply", 0)
	runcontrol.ClearStepReport("t-apply")
	runcontrol.RecordPlanToolCall("t-apply", "SetExecutorStep")

	runcontrol.AutoSubmitAfterSkillResult(ctx, "真实摄像头描述内容足够长以便通过后续 artifact 校验占位检测逻辑", []string{"SetExecutorStep"})

	ApplyStepReport(ctx, runcontrol.StepReport{
		Status:  "fail",
		Summary: `已接收：后台异步执行内置技能 "SeeCameraAndDescribe"。请通过 exec 状态/结果观察进展。`,
	})

	rep, ok := runcontrol.PeekStepReport("t-apply")
	if !ok || rep.Status != "ok" {
		t.Fatalf("want ok preserved, got %+v", rep)
	}
}

func TestApplyStepReport_upgradeFailFromCache(t *testing.T) {
	turnID := "t-cache-up"
	runcontrol.ClearStepReport(turnID)
	runcontrol.RecordPlanToolCall(turnID, "SetExecutorStep")
	skillwait.RecordResult(turnID, "缓存中的完整画面描述文本内容超过占位摘要长度用于验收")

	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, turnID, 0)

	ApplyStepReport(ctx, runcontrol.StepReport{
		Status:  "fail",
		Summary: "只收到已接收确认，技能尚未返回",
	})

	rep, ok := runcontrol.PeekStepReport(turnID)
	if !ok || rep.Status != "ok" || !strings.Contains(rep.Summary, "缓存中的完整画面") {
		t.Fatalf("want upgraded ok from cache, got %+v", rep)
	}
}
