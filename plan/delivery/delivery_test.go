package delivery

import "testing"

func TestResolveStepDisplay_PrefersSummaryOverProcessVisible(t *testing.T) {
	summary := "## MCP 列表\n| sqlite | 6 tools |\n| filesystem | 14 tools |"
	visible := "我来执行本步：先调用 list_agent_capabilities 获取第一层能力目录。"
	got := ResolveStepDisplay(summary, visible)
	if got != summary {
		t.Fatalf("want summary, got %q", got)
	}
}

func TestResolveStepDisplay_PrefersSubstantiveVisible(t *testing.T) {
	summary := "短摘要"
	visible := "完整正文段落，包含给用户的多段说明与具体数据，说明当前环境挂载的外挂能力包名称、用途与关系。\n\n## 外挂 SKILL\n- bug-pattern-diagnosis：病例库\n- complex-bug-debugging-with-ai：诊疗手册\n- demo_external：演示包\n\n以上为用户可见的完整交付内容。"
	got := ResolveStepDisplay(summary, visible)
	if got != visible {
		t.Fatalf("want visible, got %q", got)
	}
}

func TestIsProcessLike(t *testing.T) {
	if !IsProcessLike("好的，我来获取三个外挂SKILL包的完整详细信息。") {
		t.Fatal("expected process")
	}
	if IsProcessLike("当前共 3 个 MCP：sqlite、filesystem、resend。") {
		t.Fatal("expected answer")
	}
}
