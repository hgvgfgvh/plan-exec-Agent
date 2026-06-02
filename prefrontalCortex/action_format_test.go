package prefrontalCortex

import (
	"strings"
	"testing"
)

func TestExtractActionBlocks_XMLAction(t *testing.T) {
	raw := `好的，我来查看表。

<Action>sqlite__list_tables</Action>
<Action Input>{}</Action Input>`
	acts, ok := extractActionBlocks(raw)
	if !ok || len(acts) != 1 {
		t.Fatalf("expected 1 action, got ok=%v len=%d", ok, len(acts))
	}
	if acts[0].Name != "sqlite__list_tables" || acts[0].Params != "{}" {
		t.Fatalf("unexpected: %+v", acts[0])
	}
}

func TestNormalizeDSMLToReAct(t *testing.T) {
	raw := `<｜｜DSML｜｜tool_calls>
<｜｜DSML｜｜invoke name="sqlite__list_tables">
</｜｜DSML｜｜invoke>
</｜｜DSML｜｜tool_calls>`
	out := normalizeReActFormat(raw)
	acts, ok := extractActionBlocks(out)
	if !ok || len(acts) != 1 || acts[0].Name != "sqlite__list_tables" {
		t.Fatalf("got ok=%v acts=%+v out=%q", ok, acts, out)
	}
}

func TestNormalizeDSMLToReAct_WithParams(t *testing.T) {
	raw := `<｜｜DSML｜｜invoke name="get_capability_details">
<｜｜DSML｜｜parameter name="mcp_tools" string="true">filesystem</｜｜DSML｜｜parameter>
</｜｜DSML｜｜invoke>`
	out := normalizeReActFormat(raw)
	acts, ok := extractActionBlocks(out)
	if !ok || len(acts) != 1 || acts[0].Name != "get_capability_details" {
		t.Fatalf("unexpected: ok=%v acts=%+v", ok, acts)
	}
	if !strings.Contains(acts[0].Params, "filesystem") {
		t.Fatalf("params=%s", acts[0].Params)
	}
}

func TestExtractActionBlocks_Standard(t *testing.T) {
	raw := `Action: sqlite__read_query
Action Input: {"query":"SELECT 1"}`
	acts, ok := extractActionBlocks(raw)
	if !ok || len(acts) != 1 || acts[0].Name != "sqlite__read_query" {
		t.Fatalf("unexpected: ok=%v acts=%+v", ok, acts)
	}
}
