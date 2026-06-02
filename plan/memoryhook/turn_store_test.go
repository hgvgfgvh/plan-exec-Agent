package memoryhook

import (
	"AgentTest/config"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingStorer struct {
	mu      sync.Mutex
	called  bool
	content string
}

func (r *recordingStorer) Name() string { return "record" }

func (r *recordingStorer) Retrieve(context.Context, RetrieveRequest) (Experience, error) {
	return Experience{}, nil
}

func (r *recordingStorer) StoreEpisode(_ context.Context, _ EpisodeStoreInput, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	r.content = content
	return nil
}

func TestBuildEpisodeContent_andParsePlanID(t *testing.T) {
	reply := "已列出目录。\n\n---\n（编排 my-plan-id · completed · 2 步 · 记录: WorkSpace/ToDoList/my-plan-id.json）"
	in := TurnStoreInput{
		TurnID: "t-1", UserInput: "列出 WorkSpace 目录", AssistantReply: reply,
	}
	body := BuildEpisodeContent(in)
	if parsePlanIDFromReply(reply) != "my-plan-id" {
		t.Fatal("plan id parse")
	}
	if !strings.Contains(body, "列出 WorkSpace") || !strings.Contains(body, "已列出目录") {
		t.Fatalf("content: %s", body)
	}
}

func TestStoreTurnAfterProcess_skipsWhenStoreDisabled(t *testing.T) {
	falseVal := false
	rec := &recordingStorer{}
	h := &MemoryMCPHook{cfg: &config.App{}, provider: rec}
	h.cfg.PlanMemoryHook.Enabled = true
	h.cfg.PlanMemoryHook.StoreEnabled = &falseVal
	h.StoreTurnAfterProcess(context.Background(), TurnStoreInput{
		TurnID: "t-1", UserInput: "列出 WorkSpace 目录内容", AssistantReply: "ok",
	})
	time.Sleep(50 * time.Millisecond)
	rec.mu.Lock()
	called := rec.called
	rec.mu.Unlock()
	if called {
		t.Fatal("expected no store when store_enabled=false")
	}
}

func TestStoreTurnAfterProcess_callsStorerWhenEnabled(t *testing.T) {
	rec := &recordingStorer{}
	h := &MemoryMCPHook{
		cfg:      &config.App{},
		provider: rec,
	}
	h.cfg.PlanMemoryHook.Enabled = true
	h.StoreTurnAfterProcess(context.Background(), TurnStoreInput{
		TurnID: "t-2", UserInput: "列出 WorkSpace 目录内容", AssistantReply: "done",
	})
	time.Sleep(100 * time.Millisecond)
	rec.mu.Lock()
	called := rec.called
	rec.mu.Unlock()
	if !called {
		t.Fatal("expected store call")
	}
}
