package prefrontalCortex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"AgentTest/config"
)

type QwenModel struct {
	APIKey   string
	Endpoint string
	ModelID  string
}

// NewQwenModel 从 config/app.yaml integrations.dashscope 读取通义千问配置。
func NewQwenModel() *QwenModel {
	ds := config.Get().Integrations.DashScope
	return &QwenModel{
		Endpoint: strings.TrimSpace(ds.ChatCompletionsEndpoint),
		APIKey:   strings.TrimSpace(ds.APIKey),
		ModelID:  strings.TrimSpace(ds.ChatModel),
	}
}

func (m *QwenModel) Chat(input string) (string, error) {
	if strings.TrimSpace(m.APIKey) == "" {
		return "", fmt.Errorf("DashScope API key 未配置：请在 config/app.yaml 设置 integrations.dashscope.api_key")
	}
	// 1. 构造请求数据
	requestData := ChatRequest{
		Model: m.ModelID,
		Messages: []Message{
			{Role: "user", Content: input},
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	// 2. 创建 Request 对象
	req, err := http.NewRequest("POST", m.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// 设置必要的 Header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.APIKey)

	// 3. 发送请求
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// 4. 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal failed: %v", err)
	}

	if len(result.Choices) > 0 {
		content := strings.TrimSpace(result.Choices[0].Message.Content)
		reasoning := strings.TrimSpace(result.Choices[0].Message.ReasoningContent)
		if content != "" {
			return content, nil
		}
		if reasoning != "" {
			return reasoning, nil
		}
	}

	return "", fmt.Errorf("no content returned in choices")
}
