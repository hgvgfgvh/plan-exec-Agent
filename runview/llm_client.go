package runview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatPayload struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s Settings) llmConfigured() bool {
	return strings.TrimSpace(s.LLMAPIBase) != "" &&
		strings.TrimSpace(s.LLMAPIKey) != "" &&
		strings.TrimSpace(s.LLMModel) != ""
}

func normalizeChatCompletionsEndpoint(base string) string {
	b := strings.TrimSpace(base)
	b = strings.TrimRight(b, "/")
	if strings.HasSuffix(b, "/chat/completions") {
		return b
	}
	return b + "/chat/completions"
}

func chatCompletion(ctx context.Context, s Settings, system, user string) (string, error) {
	if !s.llmConfigured() {
		return "", fmt.Errorf("run_view LLM 未配置：须设置 llm_api_base、llm_api_key、llm_model")
	}
	endpoint := normalizeChatCompletionsEndpoint(s.LLMAPIBase)
	payload := chatPayload{
		Model: strings.TrimSpace(s.LLMModel),
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	timeout := time.Duration(s.LLMTimeoutSec) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.LLMAPIKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, truncateForErr(string(raw), 400))
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("llm api: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm empty choices")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("llm empty content")
	}
	return text, nil
}

func truncateForErr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
