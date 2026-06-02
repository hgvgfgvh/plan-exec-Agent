package runcontrol

import (
	"AgentTest/plan/delivery"
	"strings"
	"sync"
)

// StepReport 为 Plan 单步的结构化回执（由 report_step_result 工具提交）。
type StepReport struct {
	Status  string `json:"status"` // ok | fail
	Summary string `json:"summary"`
	// UserVisible 为本步模型写给用户的完整正文（由 Executor 从 assistant 消息捕获，非 report 短摘要）。
	UserVisible string   `json:"user_visible,omitempty"`
	ToolsCalled []string `json:"tools_called,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
}

type stepReportEntry struct {
	mu          sync.Mutex
	report      StepReport
	userVisible string
	toolCalls   []string
}

var stepReports sync.Map // turnID -> *stepReportEntry

// ClearStepReport 新 Plan 单步开始前清空本回合回执与工具调用记录。
func ClearStepReport(turnID string) {
	if turnID == "" {
		return
	}
	stepReports.Delete(turnID)
}

// RecordPlanToolCall 记录 Plan 单步内已调用的工具名（不含 report_step_result）。
func RecordPlanToolCall(turnID, toolName string) {
	if turnID == "" {
		return
	}
	name := strings.TrimSpace(toolName)
	if name == "" || name == "report_step_result" {
		return
	}
	v, _ := stepReports.LoadOrStore(turnID, &stepReportEntry{})
	e := v.(*stepReportEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, existing := range e.toolCalls {
		if existing == name {
			return
		}
	}
	e.toolCalls = append(e.toolCalls, name)
}

// MergeStepUserVisible 累积本 Plan 单步内模型输出的用户可见正文（取更长者）。
func MergeStepUserVisible(turnID, text string) {
	text = strings.TrimSpace(text)
	if turnID == "" || text == "" || text == "CONTINUE" {
		return
	}
	v, _ := stepReports.LoadOrStore(turnID, &stepReportEntry{})
	e := v.(*stepReportEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len([]rune(text)) > len([]rune(e.userVisible)) {
		e.userVisible = text
	}
}

// SetStepReport 写入本回合步骤回执（覆盖）。
func SetStepReport(turnID string, r StepReport) {
	if turnID == "" {
		return
	}
	v, _ := stepReports.LoadOrStore(turnID, &stepReportEntry{})
	e := v.(*stepReportEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(r.ToolsCalled) == 0 && len(e.toolCalls) > 0 {
		r.ToolsCalled = append([]string(nil), e.toolCalls...)
	}
	if strings.TrimSpace(r.UserVisible) == "" && strings.TrimSpace(e.userVisible) != "" {
		r.UserVisible = e.userVisible
	}
	e.report = r
}

// PeekStepReport 读取回执但不删除。
func PeekStepReport(turnID string) (StepReport, bool) {
	if turnID == "" {
		return StepReport{}, false
	}
	v, ok := stepReports.Load(turnID)
	if !ok {
		return StepReport{}, false
	}
	e := v.(*stepReportEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	if strings.TrimSpace(e.report.Summary) == "" && strings.TrimSpace(e.report.Status) == "" {
		return StepReport{}, false
	}
	return e.report, true
}

// StepReportUserText 返回应展示给用户的正文（由 plan/delivery 策略解析，非简单优先 UserVisible）。
func StepReportUserText(r StepReport) string {
	return delivery.ResolveStepDisplay(r.Summary, r.UserVisible)
}

// TakeStepReport 读取并清除本回合回执。
func TakeStepReport(turnID string) (StepReport, bool) {
	if turnID == "" {
		return StepReport{}, false
	}
	v, ok := stepReports.LoadAndDelete(turnID)
	if !ok {
		return StepReport{}, false
	}
	e := v.(*stepReportEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	if strings.TrimSpace(e.report.Summary) == "" && strings.TrimSpace(e.report.Status) == "" {
		return StepReport{}, false
	}
	return e.report, true
}
