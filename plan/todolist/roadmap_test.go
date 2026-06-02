package todolist

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRoadmapForExec(t *testing.T) {
	doc := &Document{
		ID: "t-1",
		Steps: []Step{
			{
				ID: "1", Title: "摄像头", Status: StepCompleted,
				ResultSummary: "室内明亮",
				Artifacts:     []string{"WorkSpace/cam.txt"},
				ToolsCalled:   []string{"SetExecutorStep"},
				UpdatedAt:     time.Now(),
			},
			{ID: "2", Title: "发邮件", Status: StepPending, UpdatedAt: time.Now()},
		},
	}
	road := FormatRoadmapForExec(doc, 1)
	if road == "" {
		t.Fatal("expected roadmap")
	}
	for _, p := range []string{"步骤 1", "cam.txt", "SetExecutorStep", "室内明亮"} {
		if !strings.Contains(road, p) {
			t.Fatalf("roadmap missing %q:\n%s", p, road)
		}
	}
}
