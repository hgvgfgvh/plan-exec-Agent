package prefrontalCortex

import (
	"encoding/json"
	"testing"
)

func TestExtractBalancedJSONObject_SQLQuotes(t *testing.T) {
	raw := `{"query":"SELECT * FROM t WHERE name=\"foo\" AND x='bar'}"}`
	got, ok := extractBalancedJSONObject(raw)
	if !ok || got != raw {
		t.Fatalf("ok=%v got=%q", ok, got)
	}
}

func TestExtractActionBlocks_NestedBraces(t *testing.T) {
	raw := `Action: sqlite__read_query
Action Input: {"query":"SELECT 1 WHERE json='{\"a\":1}'"}`
	acts, ok := extractActionBlocks(raw)
	if !ok || len(acts) != 1 {
		t.Fatalf("ok=%v acts=%+v", ok, acts)
	}
	if acts[0].Name != "sqlite__read_query" {
		t.Fatalf("name=%s", acts[0].Name)
	}
	if !json.Valid([]byte(acts[0].Params)) {
		t.Fatalf("invalid json params: %s", acts[0].Params)
	}
}

func TestNormalizeToolCall_SkillEnvelope(t *testing.T) {
	name, params, err := normalizeToolCall("", `{"_call":"skill","name":"SeeAndOCR","args":[]}`, nil)
	if err != nil || name != "SetExecutorStep" {
		t.Fatalf("err=%v name=%s params=%s", err, name, params)
	}
}

func TestNormalizeToolCall_MCPEnvelope(t *testing.T) {
	name, params, err := normalizeToolCall("", `{"_call":"mcp","name":"sqlite__list_tables","params":{}}`, nil)
	if err != nil || name != "sqlite__list_tables" || params != "{}" {
		t.Fatalf("err=%v name=%s params=%s", err, name, params)
	}
}

func TestNormalizeToolCall_ResendToArray(t *testing.T) {
	_, params, err := normalizeToolCall("resend__send_email", `{"to":"a@b.com","subject":"x","from":"onboarding@resend.dev"}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(params), &m); err != nil {
		t.Fatal(err)
	}
	to, ok := m["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "a@b.com" {
		t.Fatalf("to=%v", m["to"])
	}
}

func TestExtractBareResendSendJSON(t *testing.T) {
	raw := "Thought: I have the answer now.\n```json\n{\"from\":\"onboarding@resend.dev\",\"to\":[\"2563726816@qq.com\"],\"subject\":\"测试邮件\",\"text\":\"hi\"}\n```"
	acts, ok := extractActionBlocks(raw)
	if !ok || len(acts) != 1 || acts[0].Name != "resend__send_email" {
		t.Fatalf("ok=%v acts=%+v", ok, acts)
	}
}

func TestRepairJSONObject_TrailingComma(t *testing.T) {
	fixed, err := repairJSONObject(`{"a":1,}`)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(fixed)) {
		t.Fatalf("invalid: %s", fixed)
	}
}
