package skillwait

import "strings"

const setExecutorAckMarker = "已接收：后台异步执行内置技能"

// ObservationHasSetExecutorAck 工具观测串中是否含 SetExecutorStep 的异步回执。
func ObservationHasSetExecutorAck(obs string) bool {
	return strings.Contains(obs, setExecutorAckMarker)
}

// HasSkillResultInObservation 观测串是否已注入技能真实输出。
func HasSkillResultInObservation(obs string) bool {
	return strings.Contains(obs, "【内置技能执行结果】")
}

// MustWaitAfterToolBatch Plan 单步内本批调用了 SetExecutorStep 且尚无技能结果时应阻塞等待。
func MustWaitAfterToolBatch(toolNames []string, obs string) bool {
	if !toolBatchHasSetExecutorStep(toolNames) {
		return false
	}
	if HasSkillResultInObservation(obs) {
		return false
	}
	return true
}

func toolBatchHasSetExecutorStep(toolNames []string) bool {
	for _, n := range toolNames {
		if n == "SetExecutorStep" {
			return true
		}
	}
	return false
}

// IsPlaceholderSkillSummary 是否为「仅提交/后台执行中」类占位摘要（非真实技能输出）。
func IsPlaceholderSkillSummary(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if strings.Contains(s, setExecutorAckMarker) {
		return true
	}
	if strings.Contains(s, "请通过 exec 状态/结果观察进展") {
		return true
	}
	lower := strings.ToLower(s)
	if len([]rune(s)) < 160 {
		if strings.Contains(lower, "后台异步执行") && strings.Contains(lower, "已接收") {
			return true
		}
		if strings.Contains(lower, "尚未返回") || strings.Contains(lower, "只收到") && strings.Contains(lower, "已接收") {
			return true
		}
	}
	return false
}
