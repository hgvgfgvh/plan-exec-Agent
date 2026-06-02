package runcontrol

import "strings"

// PlanStepUsedTool 本 Plan 单步是否已调用指定工具（不含 report_step_result）。
func PlanStepUsedTool(turnID, toolName string) bool {
	if turnID == "" || strings.TrimSpace(toolName) == "" {
		return false
	}
	v, ok := stepReports.Load(turnID)
	if !ok {
		return false
	}
	e := v.(*stepReportEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, n := range e.toolCalls {
		if n == toolName {
			return true
		}
	}
	return false
}
