package agent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/plan/stepmeta"
	"AgentTest/plan/todolist"
	"context"
	"testing"
)

func TestClassifyStepOutcome_PlanRequiresReport(t *testing.T) {
	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, "turn-test", 0)

	st, sum, _ := classifyStepOutcome(ctx, "口头说已完成", nil, "hello", 1)
	if st != todolist.StepFailed {
		t.Fatalf("want failed, got %s", st)
	}
	if sum == "" {
		t.Fatal("expected summary")
	}

	runcontrol.SetStepReport("turn-test", runcontrol.StepReport{
		Status:  "ok",
		Summary: "已问候",
	})
	st, sum, _ = classifyStepOutcome(ctx, "ignored", nil, "hello", 1)
	if st != todolist.StepCompleted {
		t.Fatalf("want completed, got %s", st)
	}
}

func TestInferStepTier_GreetingInstruction(t *testing.T) {
	tier := stepmeta.InferTier("回应寒暄", "向用户友好地打招呼回复，无需调用任何工具或能力。", nil)
	if tier != 1 {
		t.Fatalf("want tier 1, got %d", tier)
	}
}

func TestClassifyStepOutcome_SynthesisInstructionUsesTier1(t *testing.T) {
	instr := "基于上一步获取的完整能力清单进行归纳说明。勿再调用任何工具，直接归纳总结。"
	if stepmeta.ResolveTier(2, "归纳呈现", instr, nil) != 1 {
		t.Fatal("ResolveTier should force tier 1 for synthesis-only instruction")
	}
	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, "turn-syn", 0)
	runcontrol.SetStepReport("turn-syn", runcontrol.StepReport{
		Status:      "ok",
		Summary:     "按类别归纳的能力说明…",
		ToolsCalled: []string{},
	})
	st, _, _ := classifyStepOutcome(ctx, "", nil, "有哪些能力", 1)
	if st != todolist.StepCompleted {
		t.Fatalf("want completed for tier1 synthesis, got %s", st)
	}
}

func TestClassifyStepOutcome_GreetingPassesGate(t *testing.T) {
	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, "turn-hi", 0)
	runcontrol.SetStepReport("turn-hi", runcontrol.StepReport{
		Status:      "ok",
		Summary:     "你好！",
		ToolsCalled: []string{},
	})
	st, _, _ := classifyStepOutcome(ctx, "", nil, "你好", 1)
	if st != todolist.StepCompleted {
		t.Fatalf("want completed for tier1 greeting, got %s", st)
	}
}

func TestClassifyStepOutcome_GateRejectsOkWithoutTools(t *testing.T) {
	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, "turn-gate", 0)
	runcontrol.SetStepReport("turn-gate", runcontrol.StepReport{
		Status:  "ok",
		Summary: "口头完成",
	})
	st, sum, _ := classifyStepOutcome(ctx, "", nil, "复杂任务", 2)
	if st != todolist.StepFailed {
		t.Fatalf("want failed, got %s", st)
	}
	if sum == "" {
		t.Fatal("expected gate failure message")
	}
}

func TestAutoSubmitAfterSkillResult(t *testing.T) {
	ctx := runcontrol.WithPlanStepExecution(context.Background())
	ctx = runcontrol.WithTurnMeta(ctx, "t1", 0)
	if !runcontrol.AutoSubmitAfterSkillResult(ctx, "摄像头描述摘要", []string{"SetExecutorStep"}) {
		t.Fatal("expected auto report")
	}
	rep, ok := runcontrol.PeekStepReport("t1")
	if !ok || rep.Summary != "摄像头描述摘要" {
		t.Fatalf("peek: %+v", rep)
	}
}
