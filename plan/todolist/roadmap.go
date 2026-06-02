package todolist

import (
	"fmt"
	"strings"
)

const (
	roadmapSummaryMaxRunes = 600
	stepDetailMaxRunes     = 24000
)

// RecordStepOutcome 将本步验收结果写入 Step（供后续路标注入与 TodoList 持久化）。
// userDetail 为给用户看的完整正文；summary 为短摘要（路标/编排用）。
func RecordStepOutcome(step *Step, summary, userDetail string, artifacts, toolsCalled []string) {
	if step == nil {
		return
	}
	step.ResultSummary = truncate(strings.TrimSpace(summary), roadmapSummaryMaxRunes)
	if d := strings.TrimSpace(userDetail); d != "" {
		step.ResultDetail = truncate(d, stepDetailMaxRunes)
	}
	if len(artifacts) > 0 {
		step.Artifacts = append([]string(nil), artifacts...)
	}
	if len(toolsCalled) > 0 {
		step.ToolsCalled = append([]string(nil), toolsCalled...)
	}
}

// FormatRoadmapForExec 为 execAgent 生成「已完成步骤路标」（不含当前及之后步骤）。
func FormatRoadmapForExec(doc *Document, currentIdx int) string {
	if doc == nil || currentIdx <= 0 {
		return ""
	}
	var b strings.Builder
	wrote := false
	for i := 0; i < currentIdx && i < len(doc.Steps); i++ {
		s := doc.Steps[i]
		switch s.Status {
		case StepCompleted, StepSkipped:
		default:
			continue
		}
		wrote = true
		b.WriteString(fmt.Sprintf("- 步骤 %d [%s] id=%s | %s\n", i+1, s.Status, s.ID, s.Title))
		if sum := strings.TrimSpace(s.ResultSummary); sum != "" {
			b.WriteString("  结果摘要: ")
			b.WriteString(truncate(sum, 400))
			b.WriteByte('\n')
		} else if sum := StepResultText(s); sum != "" {
			b.WriteString("  结果摘要: ")
			b.WriteString(truncate(sum, 400))
			b.WriteByte('\n')
		}
		if len(s.Artifacts) > 0 {
			b.WriteString("  产出文件: ")
			b.WriteString(strings.Join(s.Artifacts, ", "))
			b.WriteByte('\n')
		}
		if len(s.ToolsCalled) > 0 {
			b.WriteString("  已用工具: ")
			b.WriteString(strings.Join(s.ToolsCalled, ", "))
			b.WriteByte('\n')
		}
	}
	if !wrote {
		return ""
	}
	return b.String()
}
