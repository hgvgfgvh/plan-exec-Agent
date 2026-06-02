package runcontrol

import (
	"context"
	"testing"
)

func TestTryAutoSubmitSimpleEpisodeFinal_ok(t *testing.T) {
	turnID := "t-simple-auto"
	ClearStepReport(turnID)
	ctx := WithPlanSimpleExecution(WithTurnMeta(context.Background(), turnID, 0))
	text := "根据摄像头画面，室内光线充足，桌面上有电脑与文件。"
	rep, ok := TryAutoSubmitSimpleEpisodeFinal(ctx, text)
	if !ok || rep.Summary != text {
		t.Fatalf("autosubmit: ok=%v rep=%+v", ok, rep)
	}
}

func TestTryAutoSubmitSimpleEpisodeFinal_notSimple(t *testing.T) {
	turnID := "t-step-only"
	ClearStepReport(turnID)
	ctx := WithPlanStepExecution(WithTurnMeta(context.Background(), turnID, 0))
	if _, ok := TryAutoSubmitSimpleEpisodeFinal(ctx, "some long final answer that should not apply to plan step exec path"); ok {
		t.Fatal("expected false for plan step ctx")
	}
}

func TestTryAutoSubmitSimpleEpisodeFinal_existingReport(t *testing.T) {
	turnID := "t-has-report"
	ClearStepReport(turnID)
	SetStepReport(turnID, StepReport{Status: "ok", Summary: "已有"})
	ctx := WithPlanSimpleExecution(WithTurnMeta(context.Background(), turnID, 0))
	if _, ok := TryAutoSubmitSimpleEpisodeFinal(ctx, "不应覆盖"); ok {
		t.Fatal("expected false when report already set")
	}
}
