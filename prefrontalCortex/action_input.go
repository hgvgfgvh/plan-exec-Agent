package prefrontalCortex

import (
	"AgentTest/behavior/skill"
	"AgentTest/capabilities"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

var (
	reActionInputLabel = regexp.MustCompile(`(?i)Action\s*Input:\s*`)
	reCodeFenceJSON    = regexp.MustCompile("(?is)^```(?:json)?\\s*([\\s\\S]*?)```\\s*$")
	reTrailingComma    = regexp.MustCompile(`,\s*([}\]])`)
)

// extractBalancedJSONObject 从 s 开头（可含前导空白）提取第一个完整 JSON 对象。
func extractBalancedJSONObject(s string) (string, bool) {
	s = strings.TrimLeft(s, " \t\r\n")
	if !strings.HasPrefix(s, "{") {
		return "", false
	}
	depth := 0
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
}

func sanitizeActionInputJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "{}"
	}
	if m := reCodeFenceJSON.FindStringSubmatch(s); len(m) > 1 {
		s = strings.TrimSpace(m[1])
	}
	// 模型有时输出被再包一层 JSON 字符串
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		var inner string
		if err := json.Unmarshal([]byte(s), &inner); err == nil && strings.HasPrefix(strings.TrimSpace(inner), "{") {
			s = strings.TrimSpace(inner)
		}
	}
	return s
}

func repairJSONObject(s string) (string, error) {
	s = sanitizeActionInputJSON(s)
	if json.Valid([]byte(s)) {
		return s, nil
	}
	fixed := reTrailingComma.ReplaceAllString(s, "$1")
	if json.Valid([]byte(fixed)) {
		return fixed, nil
	}
	return s, fmt.Errorf("invalid JSON")
}

func parseActionInputObject(params string) (map[string]any, string, error) {
	fixed, err := repairJSONObject(params)
	if err != nil {
		return nil, params, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(fixed), &m); err != nil {
		return nil, fixed, err
	}
	return m, fixed, nil
}

func isRegisteredSkillName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	_, err := skill.GlobalManager.GetSkillDetail(name)
	return err == nil
}

func marshalParams(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func buildSetExecutorStepPayload(skillName string, args any) string {
	payload := map[string]any{"skill": skillName}
	if args != nil {
		payload["args"] = args
	} else {
		payload["args"] = []any{}
	}
	return marshalParams(payload)
}

// normalizeToolCall 将多种模型写法统一为执行器可用的 (工具名, JSON 参数字符串)。
func normalizeToolCall(toolName, params string, toolMap map[string]tools.Tool) (string, string, error) {
	toolName = strings.TrimSpace(toolName)
	m, fixed, err := parseActionInputObject(params)
	if err != nil {
		return toolName, sanitizeActionInputJSON(params), fmt.Errorf("Action Input JSON 无效: %w", err)
	}
	params = fixed

	// 统一信封：{"_call":"mcp","name":"sqlite__x","params":{...}} 或 {"_call":"skill","name":"SeeAndOCR","args":[]}
	if call, _ := m["_call"].(string); call != "" {
		switch strings.ToLower(strings.TrimSpace(call)) {
		case "mcp":
			n, _ := m["name"].(string)
			n = strings.TrimSpace(n)
			p := m["params"]
			if p == nil {
				p = m["arguments"]
			}
			pm, _ := p.(map[string]any)
			if pm == nil {
				pm = map[string]any{}
			}
			capabilities.CoerceMCPArguments(n, pm)
			return n, marshalParams(pm), nil
		case "skill":
			n, _ := m["name"].(string)
			args := m["args"]
			if args == nil {
				args = m["initial_args"]
			}
			return "SetExecutorStep", buildSetExecutorStepPayload(n, args), nil
		}
	}

	// params 内已有 skill 字段 → SetExecutorStep
	if sk, ok := m["skill"].(string); ok && strings.TrimSpace(sk) != "" {
		args := m["args"]
		if args == nil {
			args = m["initial_args"]
		}
		if args == nil {
			args = []any{}
		}
		return "SetExecutorStep", buildSetExecutorStepPayload(sk, args), nil
	}

	// Action 误写为内置技能名：Action: SeeAndOCR / Action Input: {} 或 {"args":[...]}
	if isRegisteredSkillName(toolName) {
		args := m["args"]
		if args == nil {
			args = m["initial_args"]
		}
		if args == nil {
			args = []any{}
		}
		return "SetExecutorStep", buildSetExecutorStepPayload(toolName, args), nil
	}

	// Action: skill / Action Input: {"name":"X","args":[]}（无 _call）
	if strings.EqualFold(toolName, "skill") {
		n, _ := m["name"].(string)
		if n == "" {
			n, _ = m["skill"].(string)
		}
		args := m["args"]
		if args == nil {
			args = []any{}
		}
		return "SetExecutorStep", buildSetExecutorStepPayload(n, args), nil
	}

	if toolName == "" {
		return "", params, fmt.Errorf("缺少工具名")
	}
	coerceMCPParamsInPlace(toolName, m)
	params = marshalParams(m)
	if _, ok := toolMap[toolName]; !ok && !strings.Contains(toolName, "__") {
		// 未知工具且不像 MCP 公开名时给出提示性错误（仍允许 MCP 动态名）
		if isRegisteredSkillName(toolName) {
			return "SetExecutorStep", buildSetExecutorStepPayload(toolName, []any{}), nil
		}
	}
	return toolName, params, nil
}

func coerceMCPParamsInPlace(toolName string, m map[string]any) {
	if m == nil || !strings.Contains(toolName, "__") {
		return
	}
	capabilities.CoerceMCPArguments(toolName, m)
}

// normalizeActionList 批量规范化工具调用。
func normalizeActionList(
	actions []struct{ Name, Params string },
	toolMap map[string]tools.Tool,
) []struct{ Name, Params string } {
	out := make([]struct{ Name, Params string }, 0, len(actions))
	for _, a := range actions {
		name, params, err := normalizeToolCall(a.Name, a.Params, toolMap)
		if err != nil {
			fmt.Printf(">>> [参数规范化] 工具 [%s] 警告: %v（仍尝试执行）\n", a.Name, err)
			name, params = a.Name, sanitizeActionInputJSON(a.Params)
		} else if name != a.Name || params != a.Params {
			fmt.Printf(">>> [参数规范化] %s -> %s | Input 已归一\n", a.Name, name)
		}
		out = append(out, struct{ Name, Params string }{Name: name, Params: params})
	}
	return out
}

func hasActionInputWithJSON(s string) bool {
	locs := reActionInputLabel.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return false
	}
	last := locs[len(locs)-1]
	rest := s[last[1]:]
	_, ok := extractBalancedJSONObject(rest)
	return ok
}

func lastActionInputEndIndex(s string) int {
	locs := reActionInputLabel.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return -1
	}
	last := locs[len(locs)-1]
	rest := s[last[1]:]
	if obj, ok := extractBalancedJSONObject(rest); ok {
		return last[1] + len(obj)
	}
	return -1
}
