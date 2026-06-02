package VisualAnalysisTool

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"AgentTest/config"

	"github.com/sashabaranov/go-openai"
)

var (
	clientSimple     *openai.Client
	modelNameSimple  string
	clientOnceSimple sync.Once
)

func initClientSimple() {
	clientOnceSimple.Do(func() {
		ds := config.Get().Integrations.DashScope
		cfg := openai.DefaultConfig(strings.TrimSpace(ds.APIKey))
		cfg.BaseURL = strings.TrimSpace(ds.OpenAICompatibleBaseURL)
		clientSimple = openai.NewClientWithConfig(cfg)
		modelNameSimple = strings.TrimSpace(ds.VisionModelSimple)
		if modelNameSimple == "" {
			modelNameSimple = "qwen-vl-plus"
		}
	})
}

// AnalyzeImageSimple 视觉分析（轻量模型，如 OCR、密集描述）。
func AnalyzeImageSimple(imagePath string, prompt string) (string, error) {
	initClientSimple()
	ctx := context.Background()

	imageDataURL, err := encodeImageToBase64(imagePath)
	if err != nil {
		return "", fmt.Errorf("VisionTool Error: 图片编码失败 - %w", err)
	}

	resp, err := clientSimple.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: modelNameSimple,
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
