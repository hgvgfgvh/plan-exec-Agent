package prefrontalCortex

import (
	"AgentTest/capabilities"
	"encoding/json"
	"regexp"
	"strings"
)

// toolNamePattern 匹配内置工具名与 MCP 公开名（含 sqlite__list_tables）。
const toolNamePattern = `[A-Za-z0-9_]+`

var (
	reXMLAction   = regexp.MustCompile(`(?i)<Action>\s*(` + toolNamePattern + `)\s*</Action>`)
	reXMLActionIn = regexp.MustCompile(`(?is)<Action\s*Input>\s*`)
	reActionLine  = regexp.MustCompile(`(?im)Action:\s*\*{0,2}\s*(` + toolNamePattern + `)\s*\*{0,2}`)
	reLooseAction = regexp.MustCompile(`(?im)^\s*Action:\s*\*{0,2}\s*(` + toolNamePattern + `)\s*\*{0,2}\s*$`)
)

// normalizeReActFormat 将模型常见的 XML/Markdown/DSML 变体统一为 Action: / Action Input: 行格式。
func normalizeReActFormat(input string) string {
	s := normalizeDSMLToReAct(input)
	s = reXMLAction.ReplaceAllString(s, "Action: $1")
	// XML Action Input：用平衡括号提取后重写
	for {
		loc := reXMLActionIn.FindStringIndex(s)
		if loc == nil {
			break
		}
		rest := s[loc[1]:]
		if obj, ok := extractBalancedJSONObject(rest); ok {
			s = s[:loc[0]] + "Action Input: " + obj + rest[len(obj):]
			continue
		}
		break
	}
	return s
}

var (
	reDSMLInvokeBlock = regexp.MustCompile(`(?is)DSML[^>]*invoke\s+name\s*=\s*["']([^"']+)["']\s*>([\s\S]*?)</[^>]*DSML[^>]*invoke>`)
	reDSMLParameter   = regexp.MustCompile(`(?is)DSML[^>]*parameter\s+name\s*=\s*["']([^"']+)["'][^>]*>([^<]*)<`)
)

// normalizeDSMLToReAct 将 DeepSeek DSML tool_calls 转为 ReAct 行（否则会被误判为最终回答）。
func normalizeDSMLToReAct(input string) string {
	if !strings.Contains(input, "DSML") {
		return input
	}
	if reActionLine.MatchString(input) {
		return input
	}
	matches := reDSMLInvokeBlock.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input
	}
	var b strings.Builder
	for _, m := range matches {
		toolName := strings.TrimSpace(m[1])
		if toolName == "" {
			continue
		}
		params := parseDSMLParameters(m[2])
		paramJSON, err := json.Marshal(params)
		if err != nil {
			paramJSON = []byte("{}")
		}
		b.WriteString("Action: ")
		b.WriteString(toolName)
		b.WriteString("\nAction Input: ")
		b.Write(paramJSON)
		b.WriteByte('\n')
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return input
	}
	return out
}

func parseDSMLParameters(body string) map[string]any {
	result := map[string]any{}
	for _, m := range reDSMLParameter.FindAllStringSubmatch(body, -1) {
		key := strings.TrimSpace(m[1])
		val := strings.TrimSpace(m[2])
		if key == "" {
			continue
		}
		result[key] = coerceDSMLParamValue(key, val)
	}
	if len(result) == 0 {
		return map[string]any{}
	}
	return result
}

func coerceDSMLParamValue(key, val string) any {
	switch key {
	case "mcp_tools", "external_skills", "builtin_skills", "rounds":
		if val == "" {
			return []string{}
		}
		return []string{val}
	default:
		return val
	}
}

// extractActionBlocks 从模型输出中解析 Action + Action Input（支持多组、嵌套 JSON）。
func extractActionBlocks(input string) ([]struct{ Name, Params string }, bool) {
	clean := normalizeReActFormat(input)
	reThink := regexp.MustCompile(`(?s)<think>.*?</think>`)
	clean = reThink.ReplaceAllString(clean, "")

	reMarkdownLabel := regexp.MustCompile(`(?i)\*+(Action|Action Input):\*+`)
	clean = reMarkdownLabel.ReplaceAllString(clean, "$1:")

	locPairs := reActionLine.FindAllStringSubmatchIndex(clean, -1)
	if len(locPairs) == 0 {
		if m := reLooseAction.FindStringSubmatch(clean); len(m) > 1 {
			return []struct{ Name, Params string }{{Name: strings.TrimSpace(m[1]), Params: "{}"}}, true
		}
		if name, params, ok := tryExtractBareResendSendEmailJSON(clean); ok {
			return []struct{ Name, Params string }{{Name: name, Params: params}}, true
		}
		return nil, false
	}

	var results []struct{ Name, Params string }
	for i, loc := range locPairs {
		name := strings.TrimSpace(clean[loc[2]:loc[3]])
		segStart := loc[1]
		segEnd := len(clean)
		if i+1 < len(locPairs) {
			segEnd = locPairs[i+1][0]
		}
		segment := clean[segStart:segEnd]
		params := "{}"
		if inLoc := reActionInputLabel.FindStringIndex(segment); inLoc != nil {
			rest := segment[inLoc[1]:]
			if obj, ok := extractBalancedJSONObject(rest); ok {
				params = obj
			}
		}
		results = append(results, struct{ Name, Params string }{Name: name, Params: params})
	}
	return results, len(results) > 0
}

// tryExtractBareResendSendEmailJSON：工具报错后模型常只回复 ```json{...}``` 而无 Action 行，补解析为 resend__send_email。
func tryExtractBareResendSendEmailJSON(input string) (name, params string, ok bool) {
	if reActionLine.MatchString(input) {
		return "", "", false
	}
	s := strings.TrimSpace(input)
	s = regexp.MustCompile(`(?im)^Thought:.*$`).ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	var objStr string
	if m := reCodeFenceJSON.FindStringSubmatch(s); len(m) > 1 {
		objStr = strings.TrimSpace(m[1])
	} else if obj, found := extractBalancedJSONObject(s); found {
		objStr = obj
	} else {
		return "", "", false
	}
	pm, _, err := parseActionInputObject(objStr)
	if err != nil || !looksLikeResendSendEmailPayload(pm) {
		return "", "", false
	}
	capabilities.CoerceMCPArguments("resend__send_email", pm)
	return "resend__send_email", marshalParams(pm), true
}

func looksLikeResendSendEmailPayload(m map[string]any) bool {
	if m == nil {
		return false
	}
	_, hasTo := m["to"]
	_, hasSubject := m["subject"]
	_, hasFrom := m["from"]
	_, hasText := m["text"]
	_, hasHTML := m["html"]
	return (hasTo || hasSubject) && (hasFrom || hasTo || hasText || hasHTML)
}
