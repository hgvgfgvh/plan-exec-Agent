package prefrontalCortex

import (
	"AgentTest/capabilities"
	"AgentTest/config"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

// UseAPIToolCalls 是否以 OpenAI 兼容 tools/tool_calls 为主路径（ReAct 文本为兜底）。默认开启。
func UseAPIToolCalls() bool {
	cfg := config.TryGet()
	if cfg == nil {
		return true
	}
	return !cfg.Executor.DisableAPIToolCalls
}

// ChatAPIMessage OpenAI 兼容多轮消息。
type ChatAPIMessage struct {
	Role             string            `json:"role"`
	Content          string            `json:"content,omitempty"`
	ReasoningContent string            `json:"reasoning_content,omitempty"`
	ToolCalls        []ChatAPIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string            `json:"tool_call_id,omitempty"`
}

type ChatAPIToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function ChatAPIFunctionCall `json:"function"`
}

type ChatAPIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatAPITool 工具定义（function）。
type ChatAPITool struct {
	Type     string             `json:"type"`
	Function ChatAPIFunctionDef `json:"function"`
}

type ChatAPIFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatCompletionRequest 发往兼容接口的请求体。
type ChatCompletionRequest struct {
	Model      string
	Messages   []ChatAPIMessage
	Tools      []ChatAPITool
	ToolChoice string
}

// ChatCompletionResult 解析后的模型输出。
type ChatCompletionResult struct {
	Content          string
	ReasoningContent string
	ToolCalls        []llms.ToolCall
	StopReason       string
}

type chatCompletionClient interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResult, error)
}

type chatCompletionStreamClient interface {
	ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, onDelta func(chunk string) error) (ChatCompletionResult, error)
}

func asChatCompletionStreamClient(m any) chatCompletionStreamClient {
	c, _ := m.(chatCompletionStreamClient)
	return c
}

func asChatCompletionClient(m any) chatCompletionClient {
	c, _ := m.(chatCompletionClient)
	return c
}

// apiModelForClient 配置里的 model key（如 deepSeek-onnx）≠ 远端 API 的 model id（如 deepseek-v4-pro）。
func apiModelForClient(configModelKey string, client any) string {
	switch c := client.(type) {
	case *ONNXQwen:
		if strings.TrimSpace(c.ModelID) != "" {
			return c.ModelID
		}
	case *QwenModel:
		if strings.TrimSpace(c.ModelID) != "" {
			return c.ModelID
		}
	}
	switch strings.ToLower(strings.TrimSpace(configModelKey)) {
	case "deepseek-onnx", "deepseek_onnx", "onnx-qwen", "onnx_qwen":
		return "deepseek-v4-pro"
	}
	return strings.TrimSpace(configModelKey)
}

// BuildAPITools 将执行器工具表转为 API tools 列表。
func BuildAPITools(toolMap map[string]tools.Tool) []ChatAPITool {
	if len(toolMap) == 0 {
		return nil
	}
	names := make([]string, 0, len(toolMap))
	for n := range toolMap {
		names = append(names, n)
	}
	// 稳定顺序
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	out := make([]ChatAPITool, 0, len(names))
	for _, name := range names {
		t := toolMap[name]
		desc := briefToolDescription(t.Description())
		if desc == "" {
			desc = "工具 " + name
		}
		params := capabilities.ToolParametersSchema(name)
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, ChatAPITool{
			Type: "function",
			Function: ChatAPIFunctionDef{
				Name:        name,
				Description: desc,
				Parameters:  params,
			},
		})
	}
	return out
}

// MessagesToAPI 将 langchaingo MessageContent 转为 API 消息（支持 system/user/assistant/tool）。
func MessagesToAPI(messages []llms.MessageContent) []ChatAPIMessage {
	var out []ChatAPIMessage
	for _, mc := range messages {
		text := extractMessageText(mc)
		switch mc.Role {
		case llms.ChatMessageTypeSystem:
			if text != "" {
				out = append(out, ChatAPIMessage{Role: "system", Content: text})
			}
		case llms.ChatMessageTypeHuman:
			if text != "" {
				out = append(out, ChatAPIMessage{Role: "user", Content: text})
			}
		case llms.ChatMessageTypeAI:
			if len(mc.Parts) > 0 {
				if tc, reasoning := extractToolCallsFromParts(mc.Parts); len(tc) > 0 {
					content := extractAssistantVisibleContent(mc.Parts)
					out = append(out, ChatAPIMessage{
						Role:             "assistant",
						Content:          content,
						ReasoningContent: reasoning,
						ToolCalls:        tc,
					})
					continue
				}
			}
			if text != "" {
				out = append(out, ChatAPIMessage{Role: "assistant", Content: text})
			}
		case llms.ChatMessageTypeTool:
			id := extractToolCallIDFromParts(mc.Parts)
			if id == "" {
				id = "call_legacy"
			}
			out = append(out, ChatAPIMessage{Role: "tool", ToolCallID: id, Content: text})
		}
	}
	return out
}

func extractMessageText(mc llms.MessageContent) string {
	var parts []string
	for _, p := range mc.Parts {
		if t, ok := p.(llms.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// toolCallsPayload 存入 MessageContent Parts 的 JSON 载体。
type toolCallsPayload struct {
	ReasoningContent string            `json:"reasoning_content,omitempty"`
	ToolCalls        []ChatAPIToolCall `json:"tool_calls"`
}

func extractToolCallsFromParts(parts []llms.ContentPart) ([]ChatAPIToolCall, string) {
	for _, p := range parts {
		t, ok := p.(llms.TextContent)
		if !ok {
			continue
		}
		text := strings.TrimSpace(t.Text)
		if !strings.Contains(text, `"tool_calls"`) {
			continue
		}
		var pl toolCallsPayload
		if err := json.Unmarshal([]byte(text), &pl); err == nil && len(pl.ToolCalls) > 0 {
			return pl.ToolCalls, pl.ReasoningContent
		}
	}
	return nil, ""
}

// extractAssistantVisibleContent 取 assistant 回合中面向用户的正文（排除 tool_calls JSON 部分）。
func extractAssistantVisibleContent(parts []llms.ContentPart) string {
	var visible []string
	for _, p := range parts {
		t, ok := p.(llms.TextContent)
		if !ok {
			continue
		}
		text := strings.TrimSpace(t.Text)
		if text == "" || strings.Contains(text, `"tool_calls"`) {
			continue
		}
		visible = append(visible, text)
	}
	return strings.TrimSpace(strings.Join(visible, "\n"))
}

func extractToolCallIDFromParts(parts []llms.ContentPart) string {
	for _, p := range parts {
		t, ok := p.(llms.TextContent)
		if !ok || !strings.HasPrefix(strings.TrimSpace(t.Text), "{\"tool_call_id\":") {
			continue
		}
		var pl struct {
			ToolCallID string `json:"tool_call_id"`
		}
		if err := json.Unmarshal([]byte(t.Text), &pl); err == nil {
			return pl.ToolCallID
		}
	}
	return ""
}

// AppendAssistantToolCallsHistory 将 API tool_calls 写入 history（供下一轮 API 请求）。
// reasoningContent 为 DeepSeek 思考模式必填回传字段，缺省会触发 API 400。
func AppendAssistantToolCallsHistory(history []llms.MessageContent, content string, calls []llms.ToolCall, reasoningContent string) []llms.MessageContent {
	apiCalls := make([]ChatAPIToolCall, 0, len(calls))
	for _, tc := range calls {
		if tc.FunctionCall == nil {
			continue
		}
		id := tc.ID
		if id == "" {
			id = "call_" + tc.FunctionCall.Name
		}
		apiCalls = append(apiCalls, ChatAPIToolCall{
			ID:   id,
			Type: "function",
			Function: ChatAPIFunctionCall{
				Name:      tc.FunctionCall.Name,
				Arguments: tc.FunctionCall.Arguments,
			},
		})
	}
	pl, _ := json.Marshal(toolCallsPayload{
		ReasoningContent: reasoningContent,
		ToolCalls:        apiCalls,
	})
	parts := []llms.ContentPart{llms.TextPart(string(pl))}
	if strings.TrimSpace(content) != "" {
		parts = append([]llms.ContentPart{llms.TextPart(content)}, parts...)
	}
	return append(history, llms.MessageContent{Role: llms.ChatMessageTypeAI, Parts: parts})
}

// AppendToolResultHistory 单条工具结果。
func AppendToolResultHistory(history []llms.MessageContent, toolCallID, content string) []llms.MessageContent {
	if toolCallID == "" {
		toolCallID = "call_legacy"
	}
	pl, _ := json.Marshal(struct {
		ToolCallID string `json:"tool_call_id"`
	}{ToolCallID: toolCallID})
	return append(history, llms.MessageContent{
		Role: llms.ChatMessageTypeTool,
		Parts: []llms.ContentPart{
			llms.TextPart(string(pl)),
			llms.TextPart(content),
		},
	})
}

// ActionsFromLLMResponse 优先 API tool_calls，其次文本 ReAct / ToolPlan。
func ActionsFromLLMResponse(resp *llms.ContentResponse, text string, toolMap map[string]tools.Tool) ([]struct{ Name, Params string }, bool) {
	if resp != nil && len(resp.Choices) > 0 {
		ch := resp.Choices[0]
		if len(ch.ToolCalls) > 0 {
			return toolCallsToActions(ch.ToolCalls), true
		}
		if ch.FuncCall != nil && strings.TrimSpace(ch.FuncCall.Name) != "" {
			return []struct{ Name, Params string }{
				{Name: ch.FuncCall.Name, Params: strings.TrimSpace(ch.FuncCall.Arguments)},
			}, true
		}
	}
	if plan, ok := ParseToolPlanJSON(text); ok && len(plan.Steps) > 0 {
		return ResolvePlanSteps(plan.Steps), true
	}
	if acts, ok := extractActionBlocks(text); ok {
		return acts, true
	}
	return nil, false
}

func toolCallsToActions(calls []llms.ToolCall) []struct{ Name, Params string } {
	var out []struct{ Name, Params string }
	for _, tc := range calls {
		if tc.FunctionCall == nil {
			continue
		}
		name := strings.TrimSpace(tc.FunctionCall.Name)
		if name == "" {
			continue
		}
		params := strings.TrimSpace(tc.FunctionCall.Arguments)
		if params == "" {
			params = "{}"
		}
		out = append(out, struct{ Name, Params string }{Name: name, Params: params})
	}
	return out
}

// LLMResponseToContentResponse 将 ChatCompletionResult 转为 langchaingo 响应。
func LLMResponseToContentResponse(r ChatCompletionResult) *llms.ContentResponse {
	content := strings.TrimSpace(r.Content)
	if content == "" {
		content = strings.TrimSpace(r.ReasoningContent)
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content:          content,
			ReasoningContent: r.ReasoningContent,
			ToolCalls:        r.ToolCalls,
			StopReason:       r.StopReason,
		}},
	}
}

func buildAPIToolChoice() string {
	return "auto"
}

func apiChatErrorHint(err error) string {
	return fmt.Sprintf("API ChatCompletion 失败: %v", err)
}
