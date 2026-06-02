package prefrontalCortex

import (
	"context"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

type stubChatClient struct {
	lastReq ChatCompletionRequest
	result  ChatCompletionResult
	err     error
}

func (s *stubChatClient) Chat(_ string) (string, error) {
	return "", nil
}

func (s *stubChatClient) ChatCompletion(_ context.Context, req ChatCompletionRequest) (ChatCompletionResult, error) {
	s.lastReq = req
	return s.result, s.err
}

func TestGenerateContent_UsesStructuredAPI(t *testing.T) {
	stub := &stubChatClient{
		result: ChatCompletionResult{Content: `{"summary":"ok","steps":[]}`},
	}
	m := NewMode("deepSeek-onnx", stub)
	msgs := []llms.MessageContent{
		{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextPart("sys")}},
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextPart("user")}},
	}
	resp, err := m.GenerateContent(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.lastReq.Messages) != 2 {
		t.Fatalf("messages=%d", len(stub.lastReq.Messages))
	}
	if stub.lastReq.Messages[0].Role != "system" || stub.lastReq.Messages[1].Role != "user" {
		t.Fatalf("roles=%v", stub.lastReq.Messages)
	}
	if len(stub.lastReq.Tools) != 0 || stub.lastReq.ToolChoice != "" {
		t.Fatalf("tools should be empty for GenerateContent, got tools=%d choice=%q", len(stub.lastReq.Tools), stub.lastReq.ToolChoice)
	}
	if resp.Choices[0].Content != `{"summary":"ok","steps":[]}` {
		t.Fatalf("content=%q", resp.Choices[0].Content)
	}
}

func TestGenerateForExecutor_ToolCallsThenTextFallback(t *testing.T) {
	stub := &stubChatClient{
		result: ChatCompletionResult{
			ToolCalls: []llms.ToolCall{{
				ID:           "c1",
				FunctionCall: &llms.FunctionCall{Name: "foo", Arguments: `{}`},
			}},
		},
	}
	m := NewMode("deepSeek-onnx", stub)
	msgs := []llms.MessageContent{
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextPart("run")}},
	}
	resp, err := m.GenerateForExecutor(context.Background(), msgs, map[string]tools.Tool{
		"foo": stubTool{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices[0].ToolCalls) != 1 {
		t.Fatalf("tool_calls=%v", resp.Choices[0].ToolCalls)
	}
	if stub.lastReq.ToolChoice != "auto" {
		t.Fatalf("tool_choice=%q", stub.lastReq.ToolChoice)
	}

	stub.result = ChatCompletionResult{Content: "Action: bar\nAction Input: {}"}
	resp, err = m.GenerateForExecutor(context.Background(), msgs, map[string]tools.Tool{"bar": stubTool{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices[0].ToolCalls) != 0 {
		t.Fatalf("expected text fallback, tool_calls=%v", resp.Choices[0].ToolCalls)
	}
	if resp.Choices[0].Content == "" {
		t.Fatal("expected adapted Action content")
	}
}

type stubTool struct{}

func (stubTool) Name() string        { return "stub" }
func (stubTool) Description() string { return "stub" }
func (stubTool) Call(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestChatCompletionHasBody(t *testing.T) {
	if chatCompletionHasBody(ChatCompletionResult{}) {
		t.Fatal("empty should be false")
	}
	if !chatCompletionHasBody(ChatCompletionResult{ReasoningContent: "think"}) {
		t.Fatal("reasoning should count")
	}
}
