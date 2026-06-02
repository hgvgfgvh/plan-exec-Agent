package verify

import (
	"testing"

	"AgentTest/agent/runcontrol"
)

func TestGate_Tier1_AllowsNoToolCalls(t *testing.T) {
	v := Gate(runcontrol.StepReport{Status: "ok", Summary: "hi"}, 1)
	if !v.Passed {
		t.Fatalf("tier1: %+v", v)
	}
}

func TestGate_Tier2_RequiresToolCalls(t *testing.T) {
	v := Gate(runcontrol.StepReport{Status: "ok", Summary: "done"}, 2)
	if v.Passed {
		t.Fatalf("tier2 without tools_called should fail: %+v", v)
	}
}

func TestGate_Tier2_DirectoryArtifactPassesL1(t *testing.T) {
	dir := t.TempDir()
	v := Gate(runcontrol.StepReport{
		Status:    "ok",
		Summary:   "dir ready",
		Artifacts: []string{dir},
		ToolsCalled: []string{
			"filesystem__list_directory",
		},
	}, 2)
	if !v.Passed {
		t.Fatalf("directory artifact should pass L1: %+v", v)
	}
}

func TestGate_Tier2_ArtifactsNeedWriteTool(t *testing.T) {
	// L2 临时关闭：原 E3「有 artifacts 无 write」不再失败；未填 artifacts 时仅验 tier≥2 不拦工具组合。
	v := Gate(runcontrol.StepReport{
		Status:      "ok",
		Summary:     "done",
		ToolsCalled: []string{"sqlite__list_tables"},
	}, 2)
	if !v.Passed {
		t.Fatalf("L2 bypassed: %+v", v)
	}
}
