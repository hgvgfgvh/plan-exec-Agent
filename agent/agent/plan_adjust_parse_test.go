package agent

import (
	"testing"
)

func TestParsePlanAdjustJSON_StringNewSteps(t *testing.T) {
	raw := `{
  "action": "replace_steps",
  "reason": "简化步骤",
  "new_steps": [
    "使用 playwright 下载图片到 WorkSpace/美女图片下载/",
    "使用 filesystem 列出目录确认 jpg 存在"
  ]
}`
	adj, err := parsePlanAdjustJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if adj.Action != "replace_steps" {
		t.Fatalf("action=%q", adj.Action)
	}
	if len(adj.NewSteps) != 2 {
		t.Fatalf("steps=%d", len(adj.NewSteps))
	}
	if adj.NewSteps[0].Instruction == "" {
		t.Fatal("expected instruction from string step")
	}
}

func TestParsePlanAdjustJSON_DescriptionObjects(t *testing.T) {
	raw := `{
  "action": "replace_steps",
  "reason": "更具体",
  "new_steps": [
    {"description": "用 playwright 截图保存"},
    {"instruction": "filesystem 验证文件"}
  ]
}`
	adj, err := parsePlanAdjustJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(adj.NewSteps) != 2 {
		t.Fatalf("steps=%d", len(adj.NewSteps))
	}
	if adj.NewSteps[0].Instruction != "用 playwright 截图保存" {
		t.Fatalf("step0 instr=%q", adj.NewSteps[0].Instruction)
	}
}

func TestParsePlanAdjustJSON_RetryNoNewSteps(t *testing.T) {
	raw := `{"action":"retry","reason":"再试一次"}`
	adj, err := parsePlanAdjustJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if adj.Action != "retry" || len(adj.NewSteps) != 0 {
		t.Fatalf("adj=%+v", adj)
	}
}
