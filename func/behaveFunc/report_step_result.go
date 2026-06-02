package behaveFunc

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/plan/planstep"
	"AgentTest/plan/soulhook"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// ReportStepResult Plan 单步结束时提交结构化回执，供 PlanAgent 验收。
type ReportStepResult struct{}

func (ReportStepResult) Name() string { return "report_step_result" }

func (ReportStepResult) Description() string {
	return `【Plan 单步必填】提交本步执行结果。输入 JSON：
{"status":"ok|fail","summary":"给用户/编排看的结论","artifacts":["WorkSpace/相对或绝对路径"],"tools_called":["可选，已调工具名"]}
status=ok 表示本步目标已真实完成；fail 表示未完成。禁止在未调用所需 MCP/技能时标 ok。
` + soulhook.ReferenceOnlyNotice
}

type reportStepResultInput struct {
	Status      string   `json:"status"`
	Summary     string   `json:"summary"`
	Artifacts   []string `json:"artifacts"`
	ToolsCalled []string `json:"tools_called"`
}

func (ReportStepResult) Call(ctx context.Context, input string) (string, error) {
	var params reportStepResultInput
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &params); err != nil {
		return "", fmt.Errorf("参数格式错误: %v；需要 {\"status\":\"ok|fail\",\"summary\":\"...\"}", err)
	}
	status := strings.ToLower(strings.TrimSpace(params.Status))
	if status != "ok" && status != "fail" {
		return "", fmt.Errorf("status 必须为 ok 或 fail")
	}
	summary := strings.TrimSpace(params.Summary)
	if summary == "" {
		return "", fmt.Errorf("summary 不能为空")
	}
	planstep.ApplyStepReport(ctx, runcontrol.StepReport{
		Status:      status,
		Summary:     summary,
		ToolsCalled: params.ToolsCalled,
		Artifacts:   params.Artifacts,
	})
	return fmt.Sprintf("步骤回执已记录（status=%s）", status), nil
}

func CreateReportStepResult() tools.Tool {
	return ReportStepResult{}
}
