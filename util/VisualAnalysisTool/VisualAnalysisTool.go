package VisualAnalysisTool

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"AgentTest/config"

	"github.com/sashabaranov/go-openai"
)

// 内部私有变量，实现单例
var (
	client     *openai.Client
	modelName  string
	clientOnce sync.Once
)

func initClient() {
	clientOnce.Do(func() {
		ds := config.Get().Integrations.DashScope
		cfg := openai.DefaultConfig(strings.TrimSpace(ds.APIKey))
		cfg.BaseURL = strings.TrimSpace(ds.OpenAICompatibleBaseURL)
		client = openai.NewClientWithConfig(cfg)
		modelName = strings.TrimSpace(ds.VisionModel)
		if modelName == "" {
			modelName = "qwen3.5-plus"
		}
	})
}

// AnalyzeImage 外部唯一调用的入口函数
// 外部调用示例: result, err := agentFunc.AnalyzeImage("C:/test.png", "识别文字")
func AnalyzeImage(imagePath string, prompt string) (string, error) {
	initClient()
	ctx := context.Background()

	// 1. 处理图片转换
	imageDataURL, err := encodeImageToBase64(imagePath)
	if err != nil {
		return "", fmt.Errorf("VisionTool Error: 图片编码失败 - %w", err)
	}

	// 2. 发起多模态推理
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: modelName,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL: imageDataURL,
							},
						},
						{
							Type: openai.ChatMessagePartTypeText,
							Text: prompt,
						},
					},
				},
			},
			MaxTokens: 2048,
		},
	)
	if err != nil {
		return "", fmt.Errorf("VisionTool Error: 接口调用失败 - %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func encodeImageToBase64(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType := "image/jpeg"
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}
