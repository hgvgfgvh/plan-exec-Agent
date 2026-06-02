// Package verify 实现 Plan 单步的 Verification Gate（硬规则 + 工具行为审计）。
package verify

import (
	"strconv"
	"strings"

	"AgentTest/agent/runcontrol"
	"AgentTest/plan/artifact"
)

// layer2AuditEnabled 控制 Layer 2 行为审计是否生效。
//
// 遗留问题（恢复 L2 前需先对齐契约，见 plan/verify/gate.go auditToolBehavior）：
//   - artifacts 在路标/prompt 中表示「供下一步读取的路径」，L2 却要求「本步必须有 write 类工具」，
//     只读确认步（read_text_file + 已有文件）会被误杀。
//   - hasWriteLikeTool 仅靠工具名字符串（write/edit/create/SetExecutorStep），无法覆盖多样 MCP/技能命名。
//   - tier≥2 一律要求 tools_called 非空，与「读/handoff 引用」步骤冲突。
//
// 临时策略：L2 默认通过，仅保留 L1（验盘）。恢复时改为：写步必须有 artifacts；读/handoff 步允许 artifacts 无 write。
const layer2AuditEnabled = false

// Verdict 验收闸门结论。
type Verdict struct {
	Passed   bool
	Layer    string // hard_rules | behavior
	Failures []string
}

// Gate 在 exec 回报 ok 后执行分级校验；tier 1=轻、2=标准、3=重（与 L2 审计同档）。
func Gate(rep runcontrol.StepReport, tier int) Verdict {
	if tier < 1 {
		tier = 2
	}
	if tier > 3 {
		tier = 3
	}
	status := strings.ToLower(strings.TrimSpace(rep.Status))
	if status != "ok" {
		return Verdict{Passed: true}
	}

	var failures []string

	// Layer 1: 硬规则（artifact 存在且非占位）
	if err := artifact.ValidateReportArtifacts(rep); err != nil {
		failures = append(failures, "hard_rules: "+err.Error())
	}

	// Tier>=2：至少应记录一次工具调用（除 report_step_result 外）。
	// Tier 1 允许纯文本/归纳步 tools_called 为空（见 plan/stepmeta）。
	if tier >= 2 && len(rep.ToolsCalled) == 0 {
		failures = append(failures, "hard_rules: 未记录任何工具调用（tools_called 为空）；无法确认真实执行")
	}

	if len(failures) > 0 {
		return Verdict{Passed: false, Layer: "hard_rules", Failures: failures}
	}

	// Tier 1：仅 L1；纯文本/归纳步（plan/stepmeta）允许 tools_called 为空。
	if tier <= 1 {
		return Verdict{Passed: true, Layer: "hard_rules"}
	}

	// Layer 2: 行为审计（临时关闭，见 layer2AuditEnabled 注释）
	if !layer2AuditEnabled {
		return Verdict{Passed: true, Layer: "hard_rules"}
	}
	failures = append(failures, auditToolBehavior(rep)...)
	if len(failures) > 0 {
		return Verdict{Passed: false, Layer: "behavior", Failures: failures}
	}

	return Verdict{Passed: true, Layer: "behavior"}
}

// auditToolBehavior 在 layer2AuditEnabled=true 时执行；当前默认不调用，实现保留便于恢复。
func auditToolBehavior(rep runcontrol.StepReport) []string {
	var failures []string
	calls := rep.ToolsCalled
	if len(calls) == 0 {
		failures = append(failures, "未记录任何工具调用（除 report_step_result 外）；无法确认真实执行")
		return failures
	}

	counts := make(map[string]int)
	for _, name := range calls {
		n := strings.TrimSpace(name)
		if n == "" {
			continue
		}
		counts[n]++
	}
	for name, c := range counts {
		if c > 6 {
			failures = append(failures, "工具 "+name+" 重复调用 "+strconv.Itoa(c)+" 次，疑似死循环")
		}
	}

	if len(rep.Artifacts) > 0 && !hasWriteLikeTool(calls) {
		failures = append(failures, "声明了 artifacts 但未调用可产出文件的工具（write/edit/SetExecutorStep 等）")
	}

	return failures
}

func hasWriteLikeTool(calls []string) bool {
	for _, name := range calls {
		n := strings.ToLower(name)
		if n == "setexecutorstep" {
			return true
		}
		if strings.Contains(n, "write") || strings.Contains(n, "edit") || strings.Contains(n, "create") {
			return true
		}
	}
	return false
}
