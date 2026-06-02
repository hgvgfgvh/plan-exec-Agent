package prefrontalCortex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type chatStreamPayload struct {
	Model    string           `json:"model"`
	Messages []ChatAPIMessage `json:"messages"`
	Stream   bool             `json:"stream"`
}

type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// postChatCompletionStream OpenAI 兼容 SSE 流式；onDelta 收到 content 增量（不含 reasoning）。
func postChatCompletionStream(
	ctx context.Context,
	endpoint, apiKey string,
	req ChatCompletionRequest,
	onDelta func(chunk string) error,
	debug bool,
) (ChatCompletionResult, error) {
	payload := chatStreamPayload{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   true,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	if debug {
		fmt.Println("========== API ChatCompletion 流式请求 ==========")
		fmt.Print(formatDebugChatRequest(chatHTTPPayload{Model: payload.Model, Messages: payload.Messages}))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return ChatCompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		WriteLLMExchangeLog(ctx, LLMExchangeLog{
			Kind:       "chat_completion_stream",
			Stream:     true,
			Model:      payload.Model,
			Endpoint:   sanitizeEndpointForLog(endpoint),
			HTTPStatus: resp.StatusCode,
			Error:      fmt.Sprintf("http status=%d", resp.StatusCode),
			Request:    rawJSON(payload),
			Response:   rawJSONBytes(body),
		})
		return ChatCompletionResult{}, fmt.Errorf("http status=%d body=%s", resp.StatusCode, string(body))
	}

	var acc strings.Builder
	var stopReason string
	sc := bufio.NewScanner(resp.Body)
	const maxLine = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxLine)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return ChatCompletionResult{}, fmt.Errorf("api error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		ch := chunk.Choices[0]
		if ch.FinishReason != "" {
			stopReason = ch.FinishReason
		}
		part := ch.Delta.Content
		if part == "" {
			continue
		}
		acc.WriteString(part)
		if onDelta != nil {
			if err := onDelta(part); err != nil {
				return ChatCompletionResult{}, err
			}
		}
	}
	if err := sc.Err(); err != nil {
		WriteLLMExchangeLog(ctx, LLMExchangeLog{
			Kind:     "chat_completion_stream",
			Stream:   true,
			Model:    payload.Model,
			Endpoint: sanitizeEndpointForLog(endpoint),
			Error:    err.Error(),
			Request:  rawJSON(payload),
		})
		return ChatCompletionResult{}, err
	}
	out := ChatCompletionResult{
		Content:    strings.TrimSpace(acc.String()),
		StopReason: stopReason,
	}
	WriteLLMExchangeLog(ctx, LLMExchangeLog{
		Kind:     "chat_completion_stream",
		Stream:   true,
		Model:    payload.Model,
		Endpoint: sanitizeEndpointForLog(endpoint),
		Request:  rawJSON(payload),
		Response: rawJSON(map[string]any{
			"aggregated_content": out.Content,
			"finish_reason":      out.StopReason,
		}),
	})
	return out, nil
}
