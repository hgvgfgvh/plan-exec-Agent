package agent

import (
	"strings"
	"testing"
	"time"

	"AgentTest/plan/todolist"
)

func TestBuildStepCommand_IncludesRoadmap(t *testing.T) {
	doc := &todolist.Document{
		ID:              "plan-1",
		UserRequirement: "摄像头并发邮件",
		Steps: []todolist.Step{
			{
				ID: "1", Title: "摄像头", Status: todolist.StepCompleted,
				ResultSummary: "画面明亮",
				Artifacts:     []string{"WorkSpace/camera.txt"},
				UpdatedAt:     time.Now(),
			},
			{
				ID: "2", Title: "发邮件", Instruction: "发送描述",
				Tier: 2, Status: todolist.StepPending, UpdatedAt: time.Now(),
			},
		},
	}
	cmd := buildStepCommand(doc, &doc.Steps[1], 1)
	if !strings.Contains(cmd, "【已完成步骤路标】") {
		t.Fatalf("missing roadmap section:\n%s", cmd)
	}
	if !strings.Contains(cmd, "camera.txt") {
		t.Fatalf("missing artifact in roadmap:\n%s", cmd)
	}
}
