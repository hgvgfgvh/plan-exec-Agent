package experience

import (
	"context"
)

type ExperienceProvider interface {
	// StoreMessages 负责将截断的消息存入存储系统
	StoreExperience(ctx context.Context, query, skillTree string) error
	// Retrieve 根据查询关键词检索相关的长时记忆
	// 这里的 query 可以是原始问题，也可以是主模型生成的检索指令
	RetrieveExperience(ctx context.Context, query string) (string, error)
}
