package soulhook

import (
	"AgentTest/config"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingStorer struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingStorer) Name() string { return "recording" }

func (r *recordingStorer) RetrieveHints(context.Context, string) string { return "" }

func (r *recordingStorer) StoreDialogue(_ context.Context, _ DialogueStoreInput, content string) error {
	r.mu.Lock()
	r.calls = append(r.calls, content)
	r.mu.Unlock()
	return nil
}

func TestStoreTurnAfterProcess_skipsWhenDisabled(t *testing.T) {
	cfg := testSoulConfig(t, false, true)
	h, _ := NewSoulMCPHook(cfg, &recordingStorer{})
	h.StoreTurnAfterProcess(context.Background(), WebUITurnInput{
		TurnID: "t", UserInput: "做一个复杂任务", AssistantReply: "好的",
	})
	time.Sleep(50 * time.Millisecond)
	if rs, ok := h.provider.(*recordingStorer); ok && len(rs.calls) > 0 {
		t.Fatal("disabled should not store")
	}
}

func TestStoreTurnAfterProcess_callsStorer(t *testing.T) {
	rec := &recordingStorer{}
	cfg := testSoulConfig(t, true, true)
	h, _ := NewSoulMCPHook(cfg, rec)
	h.StoreTurnAfterProcess(context.Background(), WebUITurnInput{
		TurnID: "t", UserInput: "讨论项目 Alpha 的论文", AssistantReply: "明白",
	})
	deadline := time.Now().Add(2 * time.Second)
	for {
		rec.mu.Lock()
		n := len(rec.calls)
		rec.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for soul_store")
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if !strings.Contains(rec.calls[0], "项目 Alpha") {
		t.Fatalf("unexpected content: %q", rec.calls[0])
	}
	if strings.Contains(rec.calls[0], "TodoList") {
		t.Fatal("soul store must not include todo list")
	}
}

func testSoulConfig(t *testing.T, enabled, storeEnabled bool) *config.App {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "app.yaml")
	storeYAML := "true"
	if !storeEnabled {
		storeYAML = "false"
	}
	yaml := `root: "` + filepath.ToSlash(root) + `"
paths:
  workspace: "WorkSpace"
plan_soul_hook:
  enabled: ` + boolYAML(enabled) + `
  provider: recording
  store_enabled: ` + storeYAML + `
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	RegisterProvider("recording", func(*config.App) (Provider, error) {
		return &recordingStorer{}, nil
	})
	return cfg
}

func boolYAML(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
