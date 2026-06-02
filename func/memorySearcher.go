package _func

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

// 关键接口：定义一个内部接口来避免循环引用，或者直接引用你定义好的接口
type MemorySearcher interface {
	Retrieve(ctx context.Context, query string) ([]llms.ChatMessage, error)
}

type MemoryRetrieveTool struct {
	Searcher MemorySearcher
}

func (m MemoryRetrieveTool) Name() string {
	return "recall_memory" // 工具名称：回想记忆
}

func (m MemoryRetrieveTool) Description() string {
	return "当你需要了解过去对话中的信息、之前的约定或用户提及过的历史细节时使用。输入是你想要回想的关键词或短语。例如：{\"memory_input\": \"需要查询的记忆内容\"}"
}

func (m MemoryRetrieveTool) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		MemoryInput string `json:"memory_input"`
	}

	// 尝试解析 JSON
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 容错处理：如果模型直接传了字符串而不是 JSON，尝试直接使用
		if input != "" {
			params.MemoryInput = input
		} else {
			return "", fmt.Errorf("无效的输入格式: %v", err)
		}
	}

	// 调用你之前实现的 Retrieve 方法
	msgs, err := m.Searcher.Retrieve(ctx, params.MemoryInput)
	if err != nil {
		return "", err
	}

	if len(msgs) == 0 {
		return "在长时记忆中未找到相关记录。可以考虑调用其他具体方法来获取", nil
	}

	// 格式化输出给模型看
	var res string
	for _, msg := range msgs {
		res += fmt.Sprintf("[%s]: %s\n", msg.GetType(), msg.GetContent())
	}
	return "找到以下相关记忆：\n" + res, nil
}
