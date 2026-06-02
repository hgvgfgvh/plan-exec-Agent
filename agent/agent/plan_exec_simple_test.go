package agent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/config"
	"AgentTest/plan/memoryhook"
	"AgentTest/plan/todolist"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSimpleExecutor struct {
	report runcontrol.StepReport
	err    error
}

func (f fakeSimpleExecutor) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	if strings.TrimSpace(f.report.Status) != "" || strings.TrimSpace(f.report.Summary) != "" {
		runcontrol.SetStepReport("turn-simple", f.report)
	}
	return []interface{}{"simple raw"}, f.err
}

func loadPlanSimpleTestEnv(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "app.yaml")
	data := []byte(`root: "` + filepath.ToSlash(root) + `"
paths:
  workspace: "WorkSpace"
executor:
  exec_simple_enabled: true
  exec_simple_min_confidence: 0.75
  exec_simple_max_tier: 2
plan_memory_hook:
  enabled: true
  provider: test
`)
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	memoryhook.RegisterProvider("test", func(*config.App) (memoryhook.Provider, error) {
		return testMemoryProvider{}, nil
	})
	if err := memoryhook.InitFromConfig(cfg); err != nil {
		t.Fatal(err)
	}
	config.SetGlobal(cfg)
}

type testMemoryProvider struct{}

func (testMemoryProvider) Name() string { return "test" }

func (testMemoryProvider) Retrieve(context.Context, memoryhook.RetrieveRequest) (memoryhook.Experience, error) {
	return memoryhook.Experience{Matched: true, Confidence: 0.9, Summary: "历史成功"}, nil
}

func simpleTestDoc(tier int) *todolist.Document {
	return &todolist.Document{
		ID:              todolist.NewID("simple-test"),
		UserRequirement: "执行重复任务",
		Summary:         "重复任务",
		Status:          todolist.PlanActive,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Steps: []todolist.Step{{
			ID:          "1",
			Title:       "执行",
			Instruction: "按成功路径执行",
			Tier:        tier,
			Status:      todolist.StepPending,
			UpdatedAt:   time.Now(),
		}},
	}
}

func TestPlanUsesMemoryHookForSimpleRoute(t *testing.T) {
	loadPlanSimpleTestEnv(t)
	d := memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
		Document: simpleTestDoc(2), SimpleExecutorReady: true,
	})
	if !d.UseSimple {
		t.Fatalf("expected simple route, skip=%q", d.SkipReason)
	}
	d = memoryhook.Default().DecideRoute(context.Background(), memoryhook.RouteInput{
		Document: simpleTestDoc(3), SimpleExecutorReady: true,
	})
	if d.UseSimple {
		t.Fatal("tier 3 must not route to simple")
	}
}

func TestRunExecSimpleEpisodeSuccessCompletesDocument(t *testing.T) {
	loadPlanSimpleTestEnv(t)
	ctx := runcontrol.WithTurnMeta(context.Background(), "turn-simple", 0)
	pa := &PlanAgent{
		SimpleExecutor: fakeSimpleExecutor{report: runcontrol.StepReport{
			Status:      "ok",
			Summary:     "episode 完成",
			ToolsCalled: []string{"filesystem__write_file"},
		}},
	}
	doc := simpleTestDoc(2)
	handled, final, err := pa.runExecSimpleEpisode(ctx, doc, memoryhook.Experience{Matched: true, Confidence: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("simple success should be handled")
	}
	if doc.ExecutionMode != "simple" || doc.Status != todolist.PlanCompleted {
		t.Fatalf("unexpected doc state: mode=%s status=%s", doc.ExecutionMode, doc.Status)
	}
	if doc.Steps[0].Status != todolist.StepCompleted {
		t.Fatalf("first step should be completed, got %s", doc.Steps[0].Status)
	}
	if !strings.Contains(final, "episode 完成") {
		t.Fatalf("final should include episode summary, got %q", final)
	}
}
