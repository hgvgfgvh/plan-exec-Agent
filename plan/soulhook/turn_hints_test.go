package soulhook

import (
	"strings"
	"testing"
)

func TestCombineTurnHints_orderSoulBeforeMemory(t *testing.T) {
	out := CombineTurnHints("用户问题", "SOUL块", "MEMORY块")
	if !strings.Contains(out, TurnHintPreamble) {
		t.Fatalf("missing reference preamble: %q", out)
	}
	for _, part := range []string{"SOUL块", "MEMORY块", "用户问题"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in %q", part, out)
		}
	}
	if strings.Index(out, "SOUL块") > strings.Index(out, "MEMORY块") {
		t.Fatal("memory should follow soul")
	}
	if strings.Index(out, "用户问题") < strings.Index(out, "MEMORY块") {
		t.Fatal("user input should be after memory block")
	}
}

func TestBuildWebUIDialogueContent_noTodoList(t *testing.T) {
	s := BuildWebUIDialogueContent(WebUITurnInput{
		TurnID: "t1", UserInput: "聊论文", AssistantReply: "好的\n\n---\n（编排 plan-1 ·",
	})
	for _, bad := range []string{"TodoList", "计划终态", "step1:"} {
		if strings.Contains(s, bad) {
			t.Fatalf("soul content should not include %q: %q", bad, s)
		}
	}
	if !strings.Contains(s, "聊论文") || !strings.Contains(s, "好的") {
		t.Fatalf("missing dialogue: %q", s)
	}
}
