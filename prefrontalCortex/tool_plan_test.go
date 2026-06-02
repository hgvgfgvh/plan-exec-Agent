package prefrontalCortex

import "testing"

func TestParseToolPlanJSON(t *testing.T) {
	raw := `{"intent":"observe","steps":[{"kind":"mcp","name":"sqlite__list_tables","args":"{}"}]}`
	plan, ok := ParseToolPlanJSON(raw)
	if !ok || len(plan.Steps) != 1 {
		t.Fatalf("parse failed ok=%v steps=%d", ok, len(plan.Steps))
	}
	acts := ResolvePlanSteps(plan.Steps)
	if len(acts) != 1 || acts[0].Name != "sqlite__list_tables" {
		t.Fatalf("resolve: %+v", acts)
	}
}

func TestPlanFromActionsSetExecutor(t *testing.T) {
	acts := []struct{ Name, Params string }{
		{Name: "SetExecutorStep", Params: `{"skill":"SeeAndOCR","args":[]}`},
	}
	plan := PlanFromActions(acts)
	if len(plan.Steps) != 1 || plan.Steps[0].Kind != "skill" {
		t.Fatalf("plan: %+v", plan.Steps)
	}
}
