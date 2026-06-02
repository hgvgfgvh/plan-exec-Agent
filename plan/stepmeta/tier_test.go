package stepmeta

import "testing"

func TestAllowsNoToolExecution_SynthesisPhrases(t *testing.T) {
	instr := "基于上一步获取的完整能力清单，用中文按能力类别进行归纳说明。勿再调用 list_agent_capabilities 或其它工具，直接基于上一步结果进行归纳总结。"
	if !AllowsNoToolExecution(instr) {
		t.Fatal("expected synthesis-only step")
	}
}

func TestResolveTier_ForcesTier1WhenNoTools(t *testing.T) {
	instr := "勿再调用任何工具，直接归纳并呈现给用户。"
	got := ResolveTier(2, "补充说明", instr, nil)
	if got != 1 {
		t.Fatalf("want tier 1, got %d", got)
	}
}

func TestResolveTier_KeepsTier2WhenToolsExpected(t *testing.T) {
	instr := "调用 list_agent_capabilities 获取能力列表并汇报。"
	got := ResolveTier(2, "列出能力", instr, []string{"list_agent_capabilities"})
	if got != 2 {
		t.Fatalf("want tier 2, got %d", got)
	}
}
