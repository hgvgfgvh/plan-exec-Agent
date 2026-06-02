package runcontrol

import "testing"

func TestStepReportUserTextPrefersSubstantiveVisible(t *testing.T) {
	r := StepReport{
		Summary:     "短摘要",
		UserVisible: "完整正文段落，说明当前环境能力。\n\n## 列表\n- item1：说明一\n- item2：说明二\n- item3：说明三\n包含具体数据，足以作为最终交付给用户阅读。",
	}
	if got := StepReportUserText(r); got != r.UserVisible {
		t.Fatalf("got %q", got)
	}
}

func TestStepReportUserText_PrefersSummaryWhenVisibleIsProcess(t *testing.T) {
	r := StepReport{
		Summary:     "## MCP\n| sqlite | 6 |",
		UserVisible: "我来执行本步：先调用 list_agent_capabilities。",
	}
	if got := StepReportUserText(r); got != r.Summary {
		t.Fatalf("got %q", got)
	}
}

func TestStepReportRoundTrip(t *testing.T) {
	ClearStepReport("t1")
	RecordPlanToolCall("t1", "filesystem__write_file")
	SetStepReport("t1", StepReport{Status: "ok", Summary: "done"})
	rep, ok := TakeStepReport("t1")
	if !ok {
		t.Fatal("expected report")
	}
	if rep.Status != "ok" || rep.Summary != "done" {
		t.Fatalf("report: %+v", rep)
	}
	if len(rep.ToolsCalled) != 1 || rep.ToolsCalled[0] != "filesystem__write_file" {
		t.Fatalf("tools: %v", rep.ToolsCalled)
	}
	if _, ok := PeekStepReport("t1"); ok {
		t.Fatal("Take should clear")
	}
}
