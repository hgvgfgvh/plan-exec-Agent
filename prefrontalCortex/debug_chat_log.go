package prefrontalCortex

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	debugContentMaxRunes = 600
	debugSchemaMaxRunes  = 200
)

// formatDebugChatRequest 生成可读的请求摘要（不修改实际发往 API 的 payload）。
func formatDebugChatRequest(payload chatHTTPPayload) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("model: %s\n", payload.Model))
	if payload.ToolChoice != nil {
		sb.WriteString(fmt.Sprintf("tool_choice: %v\n", payload.ToolChoice))
	}
	sb.WriteString(fmt.Sprintf("messages (%d):\n", len(payload.Messages)))
	for i, m := range payload.Messages {
		sb.WriteString(formatDebugMessageLine(i, m))
	}
	if len(payload.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("tools (%d):\n", len(payload.Tools)))
		for _, t := range payload.Tools {
			sb.WriteString(formatDebugToolLine(t))
		}
	}
	view := buildDebugRequestView(payload)
	if b, err := json.MarshalIndent(view, "", "  "); err == nil {
		sb.WriteString("\n--- 请求体（字段已截断，仅供调试）---\n")
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func formatDebugMessageLine(i int, m ChatAPIMessage) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("  [%d] %s", i, m.Role))
	if id := strings.TrimSpace(m.ToolCallID); id != "" {
		parts = append(parts, "id="+id)
	}
	if c := strings.TrimSpace(m.Content); c != "" {
		parts = append(parts, fmt.Sprintf("content(%d)=%q", utf8.RuneCountInString(c), truncateForDebug(c, 120)))
	}
	if r := strings.TrimSpace(m.ReasoningContent); r != "" {
		parts = append(parts, fmt.Sprintf("reasoning(%d)=%q", utf8.RuneCountInString(r), truncateForDebug(r, 80)))
	}
	if len(m.ToolCalls) > 0 {
		names := make([]string, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			n := strings.TrimSpace(tc.Function.Name)
			if n == "" {
				n = "?"
			}
			names = append(names, n)
		}
		parts = append(parts, "tool_calls=["+strings.Join(names, ", ")+"]")
	}
	return strings.Join(parts, " ") + "\n"
}

func formatDebugToolLine(t ChatAPITool) string {
	name := strings.TrimSpace(t.Function.Name)
	desc := truncateForDebug(strings.TrimSpace(t.Function.Description), 100)
	schemaNote := summarizeJSONSchema(t.Function.Parameters)
	return fmt.Sprintf("  - %s: %s (%s)\n", name, desc, schemaNote)
}

func summarizeJSONSchema(schema map[string]any) string {
	if len(schema) == 0 {
		return "schema=empty"
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return "schema=?"
	}
	s := string(b)
	n := utf8.RuneCountInString(s)
	if n <= debugSchemaMaxRunes {
		return "schema=" + s
	}
	return fmt.Sprintf("schema=%d runes, keys=%s", n, schemaTopKeys(schema))
}

func schemaTopKeys(schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return "—"
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	if len(keys) > 8 {
		return strings.Join(keys[:8], ",") + ",…"
	}
	return strings.Join(keys, ",")
}

type debugRequestView struct {
	Model      string             `json:"model"`
	Messages   []debugMessageView `json:"messages"`
	Tools      []debugToolView    `json:"tools,omitempty"`
	ToolChoice any                `json:"tool_choice,omitempty"`
}

type debugMessageView struct {
	Role             string            `json:"role"`
	Content          string            `json:"content,omitempty"`
	ReasoningContent string            `json:"reasoning_content,omitempty"`
	ToolCalls        []ChatAPIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string            `json:"tool_call_id,omitempty"`
}

type debugToolView struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SchemaNote  string `json:"schema_note,omitempty"`
	} `json:"function"`
}

func buildDebugRequestView(payload chatHTTPPayload) debugRequestView {
	out := debugRequestView{
		Model:      payload.Model,
		ToolChoice: payload.ToolChoice,
	}
	out.Messages = make([]debugMessageView, 0, len(payload.Messages))
	for _, m := range payload.Messages {
		out.Messages = append(out.Messages, debugMessageView{
			Role:             m.Role,
			Content:          truncateForDebug(m.Content, debugContentMaxRunes),
			ReasoningContent: truncateForDebug(m.ReasoningContent, debugContentMaxRunes),
			ToolCalls:        m.ToolCalls,
			ToolCallID:       m.ToolCallID,
		})
	}
	if len(payload.Tools) > 0 {
		out.Tools = make([]debugToolView, 0, len(payload.Tools))
		for _, t := range payload.Tools {
			v := debugToolView{Type: t.Type}
			v.Function.Name = t.Function.Name
			v.Function.Description = truncateForDebug(t.Function.Description, 200)
			v.Function.SchemaNote = summarizeJSONSchema(t.Function.Parameters)
			out.Tools = append(out.Tools, v)
		}
	}
	return out
}

func formatDebugChatResponse(body []byte) string {
	var pretty json.RawMessage
	if err := json.Unmarshal(body, &pretty); err == nil {
		if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			s := string(b)
			if utf8.RuneCountInString(s) > 8000 {
				return truncateForDebug(s, 8000) + "\n…(响应过长已截断)\n"
			}
			return s + "\n"
		}
	}
	s := string(body)
	if utf8.RuneCountInString(s) > 8000 {
		return truncateForDebug(s, 8000) + "\n…(响应过长已截断)\n"
	}
	return s + "\n"
}

func truncateForDebug(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + fmt.Sprintf("…(+%d)", len(r)-maxRunes)
}
