package prefrontalCortex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"AgentTest/config"
)

// =====================
// 请求结构
// =====================

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// =====================
// 响应结构
// =====================

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`

	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// =====================
// 主体结构
// =====================

type ONNXQwen struct {
	APIKey   string
	Endpoint string
	ModelID  string
	Debug    bool
}

func NewONNXModelQwen() *ONNXQwen {
	ds := config.Get().Integrations.DeepSeekLegacy
	return &ONNXQwen{
		Endpoint: strings.TrimSpace(ds.ChatCompletionsEndpoint),
		APIKey:   strings.TrimSpace(ds.APIKey),
		ModelID:  strings.TrimSpace(ds.Model),
		Debug:    ds.Debug,
	}
}

// =====================
// Chat 主逻辑
// =====================

func (m *ONNXQwen) Chat(input string) (string, error) {

	if input == "" {
		return "", errors.New("input is empty")
	}

	// 构造请求
	requestData := ChatRequest{
		Model: m.ModelID,
		Messages: []Message{
			{Role: "user", Content: input},
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("❌ json marshal failed: %w", err)
	}

	if m.Debug {
		fmt.Println("========== 请求参数 ==========")
		if b, err := json.MarshalIndent(requestData, "", "  "); err == nil {
			fmt.Println(string(b))
		} else {
			fmt.Println(string(jsonData))
		}
	}

	// 使用 context 控制超时
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", m.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("❌ create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.APIKey)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {

		// 网络层错误分类
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("⏰ request timeout: %w", err)
		}

		var netErr net.Error
		if errors.As(err, &netErr) {
			return "", fmt.Errorf("🌐 network error: %v", netErr)
		}

		return "", fmt.Errorf("❌ http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("❌ read response failed: %w", err)
	}

	WriteLLMExchangeLog(ctx, LLMExchangeLog{
		Kind:       "legacy_chat",
		Model:      requestData.Model,
		Endpoint:   sanitizeEndpointForLog(m.Endpoint),
		HTTPStatus: resp.StatusCode,
		Request:    rawJSON(requestData),
		Response:   rawJSONBytes(body),
	})

	if m.Debug {
		fmt.Println("========== 原始响应 ==========")
		fmt.Print(formatDebugChatResponse(body))
	}

	// HTTP 状态码检查
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"❌ http error status=%d\nbody=%s",
			resp.StatusCode,
			string(body),
		)
	}

	// 解析响应
	var result ChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("❌ json unmarshal failed: %w\nbody=%s", err, string(body))
	}

	// API 错误字段检查
	if result.Error != nil {
		return "", fmt.Errorf(
			"🚨 API error: type=%s code=%s message=%s",
			result.Error.Type,
			result.Error.Code,
			result.Error.Message,
		)
	}

	// 业务逻辑检查
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("⚠️ empty choices in response")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	reasoning := strings.TrimSpace(result.Choices[0].Message.ReasoningContent)
	if content != "" {
		return content, nil
	}
	if reasoning != "" {
		// 部分 OpenAI 兼容接口（如 DeepSeek）会把 Action 放在 reasoning_content，content 为空
		return reasoning, nil
	}
	return "", fmt.Errorf("⚠️ empty content returned by model")
}
