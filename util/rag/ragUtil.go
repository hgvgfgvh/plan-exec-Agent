package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"AgentTest/config"

	"github.com/sashabaranov/go-openai"
)

// EmbeddingClient 向量化客户端结构体
type EmbeddingClient struct {
	client *openai.Client
	model  openai.EmbeddingModel
}

var (
	instance *EmbeddingClient
	once     sync.Once
)

func initEmbeddingClient() {
	once.Do(func() {
		ds := config.Get().Integrations.DashScope
		cfg := openai.DefaultConfig(strings.TrimSpace(ds.APIKey))
		cfg.BaseURL = strings.TrimSpace(ds.OpenAICompatibleBaseURL)
		model := strings.TrimSpace(ds.EmbeddingModel)
		if model == "" {
			model = "text-embedding-v3"
		}
		instance = &EmbeddingClient{
			client: openai.NewClientWithConfig(cfg),
			model:  openai.EmbeddingModel(model),
		}
	})
}

// GetClient 获取已初始化的客户端实例（首次调用时从 config 加载）。
func GetClient() (*EmbeddingClient, error) {
	initEmbeddingClient()
	if instance == nil {
		return nil, fmt.Errorf("embedding client not initialized")
	}
	return instance, nil
}

// GetEmbeddings 批量获取文本向量
func (c *EmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: c.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("dashscope_embedding_error: %v", err)
	}

	vectors := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		vectors[i] = data.Embedding
	}
	return vectors, nil
}

// GetEmbedding 单个文本获取向量 (快捷方法)
func (c *EmbeddingClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	res, err := c.GetEmbeddings(ctx, []string{text})
	if err != nil || len(res) == 0 {
		return nil, err
	}
	return res[0], nil
}
