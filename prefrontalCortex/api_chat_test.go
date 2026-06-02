package prefrontalCortex

import (
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestActionsFromLLMResponse_ToolCallsFirst(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content: "ignored",
			ToolCalls: []llms.ToolCall{{
				ID: "call_1",
				FunctionCall: &llms.FunctionCall{
					Name:      "sqlite__list_tables",
					Arguments: `{}`,
				},
			}},
		}},
	}
	acts, ok := ActionsFromLLMResponse(resp, "Action: foo", nil)
	if !ok || len(acts) != 1 {
		t.Fatalf("want 1 action, got ok=%v acts=%v", ok, acts)
	}
	if acts[0].Name != "sqlite__list_tables" {
		t.Fatalf("name=%q", acts[0].Name)
	}
}

func TestAPIModelForClient(t *testing.T) {
	if got := apiModelForClient("deepSeek-onnx", &ONNXQwen{ModelID: "deepseek-v4-pro"}); got != "deepseek-v4-pro" {
		t.Fatalf("onnx client want deepseek-v4-pro, got %q", got)
	}
	if got := apiModelForClient("deepSeek-onnx", nil); got != "deepseek-v4-pro" {
		t.Fatalf("alias map want deepseek-v4-pro, got %q", got)
	}
}

func TestMessagesToAPI_ReasoningContentRoundTrip(t *testing.T) {
	history := AppendAssistantToolCallsHistory(nil, "visible answer", []llms.ToolCall{{
		ID: "call_1",
		FunctionCall: &llms.FunctionCall{
			Name:      "get_capability_details",
			Arguments: `{"builtin_skills":["SeeCameraAndDescribe"]}`,
		},
	}}, "chain-of-thought blob")
	msgs := MessagesToAPI(history)
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
	if msgs[0].ReasoningContent != "chain-of-thought blob" {
		t.Fatalf("reasoning=%q", msgs[0].ReasoningContent)
	}
	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("tool_calls=%v", msgs[0].ToolCalls)
	}
	if msgs[0].Content != "visible answer" {
		t.Fatalf("content=%q", msgs[0].Content)
	}
}

func TestActionsFromLLMResponse_ReActFallback(t *testing.T) {
	text := "Thought: x\nAction: list_agent_capabilities\nAction Input: {}"
	acts, ok := ActionsFromLLMResponse(nil, text, nil)
	if !ok || len(acts) != 1 || acts[0].Name != "list_agent_capabilities" {
		t.Fatalf("ok=%v acts=%v", ok, acts)
	}
}
