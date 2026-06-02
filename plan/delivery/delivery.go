// Package delivery 实现 Plan 编排层的「用户交付」策略：从 Exec 回执中选出应展示给门户的正文，
// 与 Behavior/MCP/技能执行体系解耦，不改变工具调用与渐进披露逻辑。
package delivery

import (
	"strings"
	"unicode/utf8"
)

// 过程性叙述特征（非最终交付）。命中时降低 UserVisible 优先级。
var processMarkers = []string{
	"我来执行本步",
	"我来获取",
	"现在提交本步",
	"现在提交这一步",
	"提交本步结果",
	"先调用 `",
	"先调用 list_agent",
	"获取第一层",
	"正在获取",
	"数据已获取完毕",
	"让我提交",
}

// IsProcessLike 判断文本是否主要为执行过程说明而非给用户的结果。
func IsProcessLike(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	for _, m := range processMarkers {
		if strings.Contains(t, m) {
			return true
		}
	}
	n := utf8.RuneCountInString(t)
	if n < 100 && (strings.Contains(t, "本步") || strings.Contains(t, "执行工具")) {
		return true
	}
	return false
}

// IsSubstantiveAnswer 是否像结构化交付（列表/表格/多段说明）。
func IsSubstantiveAnswer(text string) bool {
	t := strings.TrimSpace(text)
	if utf8.RuneCountInString(t) < 80 {
		return false
	}
	if strings.Contains(t, "\n##") || strings.Contains(t, "\n|") || strings.Contains(t, "\n- **") {
		return true
	}
	if strings.Count(t, "\n") >= 3 {
		return true
	}
	return utf8.RuneCountInString(t) >= 200
}

// ResolveStepDisplay 从 report summary 与 Exec 累积的 assistant 正文中选出门户应展示的单步文本。
// 原则：report_step_result.summary 为权威交付；UserVisible 仅在非过程性且更完整时优先。
func ResolveStepDisplay(summary, assistantVisible string) string {
	s := strings.TrimSpace(summary)
	a := strings.TrimSpace(assistantVisible)
	if s == "" {
		return a
	}
	if a == "" {
		return s
	}

	sRunes := utf8.RuneCountInString(s)
	aRunes := utf8.RuneCountInString(a)
	aProc := IsProcessLike(a)
	sProc := IsProcessLike(s)

	switch {
	case aProc && !sProc:
		return s
	case sProc && !aProc:
		return a
	case aProc && sProc:
		if sRunes >= aRunes {
			return s
		}
		return a
	case IsSubstantiveAnswer(s) && !IsSubstantiveAnswer(a):
		return s
	case IsSubstantiveAnswer(a) && !IsSubstantiveAnswer(s):
		return a
	case sRunes > aRunes+60:
		return s
	case aRunes > sRunes+60:
		return a
	default:
		return s
	}
}

// ShouldSynthesizeFinalReply 单步任务是否值得调用交付助手做整轮润色归纳。
func ShouldSynthesizeFinalReply(stepMaterial string, minRunes int) bool {
	if minRunes <= 0 {
		minRunes = 400
	}
	n := utf8.RuneCountInString(strings.TrimSpace(stepMaterial))
	if n < minRunes {
		return false
	}
	if IsSubstantiveAnswer(stepMaterial) {
		return true
	}
	return n >= minRunes*2
}

// StepProgressLine 生成可选的门户进度一行（不替代最终总回复）。
func StepProgressLine(stepIndex, stepTotal int, title, resultSummary string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 280
	}
	title = strings.TrimSpace(title)
	sum := strings.TrimSpace(resultSummary)
	var b strings.Builder
	b.WriteString("步骤 ")
	b.WriteString(intToStr(stepIndex))
	b.WriteString("/")
	b.WriteString(intToStr(stepTotal))
	if title != "" {
		b.WriteString(" · ")
		b.WriteString(title)
	}
	if sum != "" {
		b.WriteString(" — ")
		b.WriteString(truncateRunes(sum, maxRunes))
	}
	return b.String()
}

func intToStr(n int) string {
	if n <= 0 {
		return "0"
	}
	var d [20]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d[i:])
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
