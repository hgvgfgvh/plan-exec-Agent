package memory

import (
	"context"
	"github.com/tmc/langchaingo/llms"
)

// LongTermMemoryProvider 定义了长效记忆处理的接口
type LongTermMemoryProvider interface {
	// StoreMessages 负责将截断的消息存入存储系统
	StoreMessages(ctx context.Context, messages []llms.ChatMessage) error
	// Retrieve 根据查询关键词检索相关的长时记忆
	// 这里的 query 可以是原始问题，也可以是主模型生成的检索指令
	Retrieve(ctx context.Context, query string) ([]llms.ChatMessage, error)
	//【直觉短期记忆获取】
	GetInstinct(query string) (string, bool)
}
