package prefrontalCortex

import (
	"AgentTest/entity"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

var errNoChatCompletionClient = errors.New("chat completion client unavailable")

type Mode struct {
	DefaultModel  string
	ModelInstance entity.ONNXModel
}

func NewMode(name string, ModelInstance entity.ONNXModel) *Mode {
	return &Mode{
		DefaultModel:  name,
		ModelInstance: ModelInstance,
	}
}

// chatCompletion 统一 OpenAI 兼容 Chat Completions 调用（结构化 messages；tools 可选）。
func (m *Mode) chatCompletion(ctx context.Context, messages []llms.MessageContent, tools []ChatAPITool) (ChatCompletionResult, error) {
	if !UseAPIToolCalls() {
		return ChatCompletionResult{}, errNoChatCompletionClient
	}
	client := asChatCompletionClient(m.ModelInstance)
	if client == nil {
		return ChatCompletionResult{}, errNoChatCompletionClient
	}
	apiMsgs := MessagesToAPI(messages)
	if len(apiMsgs) == 0 {
		return ChatCompletionResult{}, fmt.Errorf("empty messages for ChatCompletion")
	}
	req := ChatCompletionRequest{
		Model:    apiModelForClient(m.DefaultModel, m.ModelInstance),
		Messages: apiMsgs,
	}
	if len(tools) > 0 {
		req.Tools = tools
		req.ToolChoice = buildAPIToolChoice()
	}
	return client.ChatCompletion(ctx, req)
}

func chatCompletionHasBody(r ChatCompletionResult) bool {
	return len(r.ToolCalls) > 0 ||
		strings.TrimSpace(r.Content) != "" ||
		strings.TrimSpace(r.ReasoningContent) != ""
}

// GenerateForExecutor 执行器入口：优先 API tool_calls，无 calls 时解析 content 文本 Action；API 不可用时回退 legacy ReAct。
func (m *Mode) GenerateForExecutor(ctx context.Context, messages []llms.MessageContent, toolMap map[string]tools.Tool) (*llms.ContentResponse, error) {
	var apiTools []ChatAPITool
	if len(toolMap) > 0 {
		apiTools = BuildAPITools(toolMap)
	}
	result, err := m.chatCompletion(ctx, messages, apiTools)
	if err == nil && chatCompletionHasBody(result) {
		if len(result.ToolCalls) > 0 {
			fmt.Println("[executor] 使用 API tool_calls 主路径")
			return LLMResponseToContentResponse(result), nil
		}
		fmt.Println("[executor] API 无 tool_calls，回退解析文本 Action")
		resp := LLMResponseToContentResponse(result)
		content := ""
		if len(resp.Choices) > 0 {
			content = resp.Choices[0].Content
		}
		return m.wrapTextAdapterResponse(ctx, content, messages)
	}
	if err != nil && !errors.Is(err, errNoChatCompletionClient) {
		fmt.Println(apiChatErrorHint(err))
		if isAPIToolLoopHistory(messages) {
			return nil, fmt.Errorf("API 多轮 tool_calls 失败，已禁止回退文本模型以免幻觉: %w", err)
		}
	}
	return m.generateContentLegacy(ctx, messages)
}

// isAPIToolLoopHistory 是否已进入 tool_calls 多轮对话（此时 API 失败不得回退 GenerateContent）。
func isAPIToolLoopHistory(messages []llms.MessageContent) bool {
	for _, mc := range messages {
		if mc.Role == llms.ChatMessageTypeTool {
			return true
		}
		if mc.Role == llms.ChatMessageTypeAI && len(mc.Parts) > 0 {
			if tc, _ := extractToolCallsFromParts(mc.Parts); len(tc) > 0 {
				return true
			}
		}
	}
	return false
}

func (m *Mode) wrapTextAdapterResponse(ctx context.Context, raw string, messages []llms.MessageContent) (*llms.ContentResponse, error) {
	_ = messages
	raw = strings.TrimSpace(normalizeReActFormat(raw))
	if end := lastActionInputEndIndex(raw); end > 0 {
		raw = strings.TrimSpace(raw[:end])
	}
	if raw == "" {
		return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: ""}}}, nil
	}
	if _, ok := extractActionBlocks(raw); ok {
		return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: raw}}}, nil
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: fmt.Sprintf("Thought: I have the answer now.\n%s", raw)}}}, nil
}

// GenerateContent 非执行器入口：优先结构化 Chat Completions（无 tools）；失败时回退 flatten+Chat 文本路径。
func (m *Mode) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	_ = options
	result, err := m.chatCompletion(ctx, messages, nil)
	if err == nil && chatCompletionHasBody(result) {
		fmt.Println("[model] 使用 API ChatCompletion 结构化主路径（无 tools）")
		return LLMResponseToContentResponse(result), nil
	}
	if err != nil && !errors.Is(err, errNoChatCompletionClient) {
		fmt.Println(apiChatErrorHint(err))
	}
	return m.generateContentLegacy(ctx, messages)
}

// GenerateContentStream 交付助手等场景：API 流式增量回调；不可流式时回退为单次 GenerateContent。
func (m *Mode) GenerateContentStream(ctx context.Context, messages []llms.MessageContent, onDelta func(chunk string) error) (string, error) {
	if onDelta == nil {
		onDelta = func(string) error { return nil }
	}
	if !UseAPIToolCalls() {
		return m.generateContentStreamLegacy(ctx, messages, onDelta)
	}
	streamClient := asChatCompletionStreamClient(m.ModelInstance)
	if streamClient == nil {
		return m.generateContentStreamLegacy(ctx, messages, onDelta)
	}
	apiMsgs := MessagesToAPI(messages)
	if len(apiMsgs) == 0 {
		return "", fmt.Errorf("empty messages for ChatCompletionStream")
	}
	req := ChatCompletionRequest{
		Model:    apiModelForClient(m.DefaultModel, m.ModelInstance),
		Messages: apiMsgs,
	}
	result, err := streamClient.ChatCompletionStream(ctx, req, onDelta)
	if err == nil && strings.TrimSpace(result.Content) != "" {
		fmt.Println("[model] 使用 API ChatCompletion 流式主路径（无 tools）")
		return result.Content, nil
	}
	if err != nil && !errors.Is(err, errNoChatCompletionClient) {
		fmt.Println(apiChatErrorHint(err))
	}
	return m.generateContentStreamLegacy(ctx, messages, onDelta)
}

func (m *Mode) generateContentStreamLegacy(ctx context.Context, messages []llms.MessageContent, onDelta func(chunk string) error) (string, error) {
	resp, err := m.GenerateContent(ctx, messages)
	if err != nil {
		return "", err
	}
	text := ""
	if resp != nil && len(resp.Choices) > 0 {
		text = strings.TrimSpace(resp.Choices[0].Content)
	}
	if text != "" {
		_ = onDelta(text)
	}
	return text, nil
}

// generateContentLegacy 旧版单条 user 拼接 + Chat()，含 Action/ToolPlan 文本解析兜底。
func (m *Mode) generateContentLegacy(ctx context.Context, messages []llms.MessageContent) (*llms.ContentResponse, error) {
	_ = ctx
	id := uuid.New()
	prompt := m.buildPrompt(messages)
	fmt.Println(id.String() + "---------------模型原始输入：---------------|/ \n" + prompt)
	fmt.Println(id.String() + "----------------模型原始输入-------------^")
	rawOutput, err := m.ModelInstance.Chat(prompt)
	fmt.Printf("\n[RAW_DEBUG] 原始输出长度: %d\n", len(rawOutput))
	fmt.Printf("[RAW_DEBUG] 原始输出内容开始 >>>\n%s\n<<< 原始输出内容结束\n", rawOutput)
	if err != nil {
		return nil, err
	}

	rawOutput = strings.TrimSpace(rawOutput)
	rawOutput = normalizeReActFormat(rawOutput)
	fmt.Println(id.String() + "--------------【模型原始返回：】-----------------|/\n" + rawOutput)

	fmt.Println(id.String() + "----------------【模型原始返回：】-------------^")
	var finalContent string

	// 1.1【鲁棒性补丁】当模型给出 Action: xxx 但忘了 Action Input（典型为无参工具）时，
	//    自动补一个空对象 Action Input: {}。否则下游 extractActions 双匹配失败，会把这条 Action 行当成
	//    最终回答原样喷给用户（曾出现："Action: list_mcp_servers" 直达用户屏幕）。
	if !hasActionInputWithJSON(rawOutput) {
		reActionLine := regexp.MustCompile(`(?m)^\s*Action:\s*` + toolNamePattern + `\s*$`)
		if reActionLine.MatchString(rawOutput) || reXMLAction.MatchString(rawOutput) {
			rawOutput = strings.TrimRight(rawOutput, " \t\r\n") + "\nAction Input: {}"
			fmt.Println("[补丁] 检测到 Action 缺失 Action Input，已自动补全为 {}（无参工具兼容）。")
		}
	}

	// 2. 按平衡括号定位最后一个 Action Input JSON 的结束位置（避免 SQL 内 } 截断错误）
	if end := lastActionInputEndIndex(rawOutput); end > 0 {
		truncatedOutput := strings.TrimSpace(rawOutput[:end])

		finalContent = truncatedOutput
		fmt.Printf("\n[拦截] 检测到 Action 模式，已基于最后一条 Action Input（平衡括号）完成截断。\n")

	} else if strings.HasPrefix(rawOutput, "{") {
		// ToolPlan JSON：{"intent":"observe","steps":[{"kind":"mcp","name":"sqlite__list_tables","args":"{}"}]}
		if plan, ok := ParseToolPlanJSON(rawOutput); ok && len(plan.Steps) > 0 {
			finalContent = rawOutput
			fmt.Println("[架构] 检测到 ToolPlan JSON，保留原样供 Executor 解析。")
		} else {
			// 兼容旧版单工具 JSON
			var toolReq struct {
				Tool       string         `json:"tool"`
				Parameters map[string]any `json:"parameters"`
			}
			if err := json.Unmarshal([]byte(rawOutput), &toolReq); err == nil && toolReq.Tool != "" {
				paramBytes, _ := json.Marshal(toolReq.Parameters)
				finalContent = fmt.Sprintf("Thought: I should call the tool.\nAction: %s\nAction Input: %s",
					toolReq.Tool, string(paramBytes))
			}
		}
	}

	// 逻辑 B: 如果既不是 Action 格式，也没被截断过，判定为普通文本
	if finalContent == "" {
		// 如果 rawOutput 中包含 <think> 标签，我们建议保留它，因为 Executor 会处理
		finalContent = fmt.Sprintf("Thought: I have the answer now.\n%s", rawOutput)
	}

	fmt.Printf("\n[DEBUG] 适配后的 Agent 输出:\n%s\n", finalContent)

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: finalContent}},
	}, nil
}

func (o *Mode) buildPrompt(messages []llms.MessageContent) string {
	var sb strings.Builder

	// 针对 DeepSeek/Llama 系列模型的标准格式处理
	for _, mc := range messages {
		content := o.extractText(mc)
		switch mc.Role {
		case llms.ChatMessageTypeSystem:
			sb.WriteString(fmt.Sprintf("<｜System｜>%s\n", content))
		case llms.ChatMessageTypeHuman:
			sb.WriteString(fmt.Sprintf("<｜User｜>%s", content))
		case llms.ChatMessageTypeAI:
			sb.WriteString(fmt.Sprintf("<｜Assistant｜>%s<｜endofsentence｜>", content))
		case llms.ChatMessageTypeTool:
			// 当智能体工具执行完，结果会通过这个 Role 传回来
			sb.WriteString(fmt.Sprintf("<｜Tool Result｜>%s\n", content))
		}
	}

	// 结尾符，提示模型该由它产生输出了
	sb.WriteString("<｜Assistant｜>")
	return sb.String()
}

// 辅助方法：从复杂的 MessageContent 中提取文本部分
func (o *Mode) extractText(mc llms.MessageContent) string {
	var textParts []string
	for _, part := range mc.Parts {
		if t, ok := part.(llms.TextContent); ok {
			textParts = append(textParts, t.Text)
		}
	}
	return strings.Join(textParts, "\n")
}

func (m *Mode) parseToolCall(rawOutput string) (llms.ToolCall, bool) {
	// 使用正则表达式匹配 <tool_call> 标签内的 JSON
	re := regexp.MustCompile(`<tool_call>(.*?)</tool_call>`)
	match := re.FindStringSubmatch(rawOutput)
	if len(match) < 2 {
		return llms.ToolCall{}, false
	}

	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(match[1]), &call); err != nil {
		return llms.ToolCall{}, false
	}

	return llms.ToolCall{
		FunctionCall: &llms.FunctionCall{
			Name:      call.Name,
			Arguments: string(call.Arguments),
		},
	}, true
}

// 实现兼容旧版本的 Call 方法（可选）
func (o *Mode) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, o, prompt, options...)
}
