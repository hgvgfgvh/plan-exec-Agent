package agent

import (
	"testing"

	"AgentTest/config"
)

func TestGetPlanOrchestrationLimits_FromConfig(t *testing.T) {
	cfg := config.Get()
	if cfg == nil {
		t.Fatal("config not loaded")
	}
	lim := getPlanOrchestrationLimits()
	if lim.maxStepsPerPlan != cfg.Executor.PlanMaxStepsPerPlan {
		t.Fatalf("maxStepsPerPlan: got %d want %d", lim.maxStepsPerPlan, cfg.Executor.PlanMaxStepsPerPlan)
	}
	if lim.maxDispatchPerTurn != cfg.Executor.PlanMaxDispatchPerTurn {
		t.Fatalf("maxDispatchPerTurn: got %d want %d", lim.maxDispatchPerTurn, cfg.Executor.PlanMaxDispatchPerTurn)
	}
	if lim.resultSummaryMaxRunes != cfg.Executor.PlanResultSummaryMaxRunes {
		t.Fatalf("resultSummaryMaxRunes: got %d want %d", lim.resultSummaryMaxRunes, cfg.Executor.PlanResultSummaryMaxRunes)
	}
}
