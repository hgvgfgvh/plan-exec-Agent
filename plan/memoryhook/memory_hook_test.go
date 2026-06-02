package memoryhook

import (
	"AgentTest/config"
	"AgentTest/plan/todolist"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stubProvider struct {
	exp Experience
	err error
}

func (s stubProvider) Name() string { return "stub" }

func (s stubProvider) Retrieve(context.Context, RetrieveRequest) (Experience, error) {
	return s.exp, s.err
}

func loadHookTestConfig(t *testing.T, hookEnabled, execSimpleEnabled bool) *config.App {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "app.yaml")
	yaml := `root: "` + filepath.ToSlash(root) + `"
paths:
  workspace: "WorkSpace"
executor:
  exec_simple_enabled: ` + boolYAML(execSimpleEnabled) + `
  exec_simple_min_confidence: 0.75
  exec_simple_max_tier: 2
plan_memory_hook:
  enabled: ` + boolYAML(hookEnabled) + `
  provider: stub
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	RegisterProvider("stub", func(*config.App) (Provider, error) {
		return stubProvider{exp: Experience{Matched: true, Confidence: 0.9, Summary: "ok"}}, nil
	})
	config.SetGlobal(cfg)
	if err := InitFromConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func boolYAML(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func testDoc(tier int) *todolist.Document {
	return &todolist.Document{
		ID:              "t1",
		UserRequirement: "req",
		Steps: []todolist.Step{{
			ID: "1", Title: "s", Tier: tier, Status: todolist.StepPending, UpdatedAt: time.Now(),
		}},
	}
}

func TestDecideRoute_DisabledWhenHookOff(t *testing.T) {
	loadHookTestConfig(t, false, true)
	d := Default().DecideRoute(context.Background(), RouteInput{Document: testDoc(1), SimpleExecutorReady: true})
	if d.UseSimple {
		t.Fatal("hook disabled should not route to simple")
	}
}

func TestDecideRoute_UsesStubProvider(t *testing.T) {
	loadHookTestConfig(t, true, true)
	d := Default().DecideRoute(context.Background(), RouteInput{Document: testDoc(2), SimpleExecutorReady: true})
	if !d.UseSimple {
		t.Fatalf("expected simple, skip=%q", d.SkipReason)
	}
	if d.Experience.Summary != "ok" {
		t.Fatalf("experience: %+v", d.Experience)
	}
}

func TestDecideRoute_TierGuard(t *testing.T) {
	loadHookTestConfig(t, true, true)
	d := Default().DecideRoute(context.Background(), RouteInput{Document: testDoc(3), SimpleExecutorReady: true})
	if d.UseSimple {
		t.Fatal("tier 3 must not use simple")
	}
}
