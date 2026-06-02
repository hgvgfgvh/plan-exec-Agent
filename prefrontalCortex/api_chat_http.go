package prefrontalCortex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type chatHTTPPayload struct {
	Model      string           `json:"model"`
	Messages   []ChatAPIMessage `json:"messages"`
	Tools      []ChatAPITool    `json:"tools,omitempty"`
	ToolChoice any              `json:"tool_choice,omitempty"`
}

type chatHTTPResponse struct {
	Choices []struct {
		Message struct {
			Content          string             `json:"content"`
			ReasoningContent string             `json:"reasoning_content"`
			ToolCalls        []chatHTTPToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type chatHTTPToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func postChatCompletion(ctx context.Context, endpoint, apiKey string, req ChatCompletionRequest, debug bool) (ChatCompletionResult, error) {
	payload := chatHTTPPayload{
		Model:    req.Model,
		Messages: req.Messages,
		Tools:    req.Tools,
	}
	if len(req.Tools) > 0 {
		if req.ToolChoice != "" {
			payload.ToolChoice = req.ToolChoice
		} else {
			payload.ToolChoice = "auto"
		}
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	if debug {
		fmt.Println("========== API ChatCompletion 请求 ==========")
		fmt.Print(formatDebugChatRequest(payload))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return ChatCompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		WriteLLMExchangeLog(ctx, LLMExchangeLog{
			Kind:     "chat_completion",
			Model:    payload.Model,
			Endpoint: sanitizeEndpointForLog(endpoint),
			Error:    err.Error(),
			Request:  rawJSON(payload),
		})
		return ChatCompletionResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		WriteLLMExchangeLog(ctx, LLMExchangeLog{
			Kind:       "chat_completion",
			Model:      payload.Model,
			Endpoint:   sanitizeEndpointForLog(endpoint),
			HTTPStatus: resp.StatusCode,
			Error:      err.Error(),
			Request:    rawJSON(payload),
		})
		return ChatCompletionResult{}, err
	}
	writeLLMChatCompletionExchange(ctx, endpoint, payload, resp.StatusCode, body)
	if debug {
		fmt.Println("========== API ChatCompletion 响应 ==========")
		fmt.Print(formatDebugChatResponse(body))
	}
	if resp.StatusCode != http.StatusOK {
		return ChatCompletionResult{}, fmt.Errorf("http status=%d body=%s", resp.StatusCode, string(body))
	}
	var result chatHTTPResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ChatCompletionResult{}, err
	}
	if result.Error != nil {
		return ChatCompletionResult{}, fmt.Errorf("api error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return ChatCompletionResult{}, fmt.Errorf("empty choices")
	}
	ch := result.Choices[0]
	out := ChatCompletionResult{
		Content:          strings.TrimSpace(ch.Message.Content),
		ReasoningContent: strings.TrimSpace(ch.Message.ReasoningContent),
		StopReason:       ch.FinishReason,
	}
	for _, tc := range ch.Message.ToolCalls {
		name := strings.TrimSpace(tc.Function.Name)
		if name == "" {
			continue
		}
		args := strings.TrimSpace(tc.Function.Arguments)
		if args == "" {
			args = "{}"
		}
		out.ToolCalls = append(out.ToolCalls, llms.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			FunctionCall: &llms.FunctionCall{
				Name:      name,
				Arguments: args,
			},
		})
	}
	return out, nil
}

func writeLLMChatCompletionExchange(ctx context.Context, endpoint string, payload chatHTTPPayload, httpStatus int, body []byte) {
	rec := LLMExchangeLog{
		Kind:       "chat_completion",
		Stream:     false,
		Model:      payload.Model,
		Endpoint:   sanitizeEndpointForLog(endpoint),
		HTTPStatus: httpStatus,
		Request:    rawJSON(payload),
		Response:   rawJSONBytes(body),
	}
	if httpStatus != http.StatusOK {
		rec.Error = fmt.Sprintf("http status=%d", httpStatus)
	}
	WriteLLMExchangeLog(ctx, rec)
}

func (m *ONNXQwen) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, onDelta func(chunk string) error) (ChatCompletionResult, error) {
	if id := strings.TrimSpace(m.ModelID); id != "" {
		req.Model = id
	} else if req.Model == "" {
		req.Model = "deepseek-v4-pro"
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
	}
	return postChatCompletionStream(ctx, m.Endpoint, m.APIKey, req, onDelta, m.Debug)
}

func (m *ONNXQwen) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResult, error) {
	if id := strings.TrimSpace(m.ModelID); id != "" {
		req.Model = id
	} else if req.Model == "" {
		req.Model = "deepseek-v4-pro"
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
	}
	return postChatCompletion(ctx, m.Endpoint, m.APIKey, req, m.Debug)
}

func (m *QwenModel) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, onDelta func(chunk string) error) (ChatCompletionResult, error) {
	if id := strings.TrimSpace(m.ModelID); id != "" {
		req.Model = id
	} else if req.Model == "" {
		req.Model = "qwen3-max"
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
	}
	return postChatCompletionStream(ctx, m.Endpoint, m.APIKey, req, onDelta, false)
}

func (m *QwenModel) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResult, error) {
	if id := strings.TrimSpace(m.ModelID); id != "" {
		req.Model = id
	} else if req.Model == "" {
		req.Model = "qwen3-max"
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
	}
	return postChatCompletion(ctx, m.Endpoint, m.APIKey, req, false)
}
