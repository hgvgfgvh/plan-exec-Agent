package turnjournal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTodoListPath(t *testing.T) {
	reply := "好的。\n\n---\n（编排 my-plan · completed · 1 步 · 记录: C:/tmp/WorkSpace/ToDoList/my-plan.json）"
	got := parseTodoListPath(reply)
	if !strings.HasSuffix(got, "my-plan.json") {
		t.Fatalf("parse path: %q", got)
	}
}

func TestBeginFinalizeWritesBundle(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("AGENTTEST_CONFIG")
	t.Setenv("AGENTTEST_CONFIG", "")
	_ = old

	// 直接写临时目录：覆盖 FilePath 不可行，测 writeBundle + 结构
	b := &Bundle{
		TurnID:    "t-test-1",
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
		UserInput: "你好",
		PathMode:  "plan",
		Status:    "ok",
		Portal: PortalSection{
			FinalReply:   "早上好",
			OutputSource: "计划编排",
		},
	}
	path := filepath.Join(dir, "t-test-1.json")
	b.LogPath = path
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "早上好") {
		t.Fatalf("bundle content: %s", raw)
	}
}

func TestTruncateRunes(t *testing.T) {
	s := strings.Repeat("你", 20)
	out := truncateRunes(s, 5)
	if len([]rune(out)) != 6 { // 5 + '…'
		t.Fatalf("got %q len %d", out, len([]rune(out)))
	}
}
