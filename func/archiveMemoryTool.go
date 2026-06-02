package _func

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
)

type MemoryArchiver interface {
	StoreMessages(ctx context.Context, msgs []llms.ChatMessage) error
}

type ArchiveMemoryTool struct {
	Archiver   MemoryArchiver
	ChatMemory *memory.ConversationBuffer // 延迟注入
}

// ExecutorParallelUnsafe 标记：该工具会写 ChatMemory.ChatHistory（非线程安全），
// CustomExecutor 发现批次内含本工具时回退为串行执行。
func (t *ArchiveMemoryTool) ExecutorParallelUnsafe() {}

func (t *ArchiveMemoryTool) Name() string {
	return "archive_specific_rounds"
}

func (t *ArchiveMemoryTool) Description() string {
	return `按需主动触发：当你发现对话历史中某些特定的轮次（Round）不是特别重要可以当前内存中移除时使用。输入参数是轮次索引列表。第一轮对话为1。例如：{"rounds": [1, 3, 5]} 表示将第1、3、5轮对话移入长期记忆并从当前内存删除。`
}

func (t *ArchiveMemoryTool) Call(ctx context.Context, input string) (string, error) {
	if t.ChatMemory == nil {
		return "", fmt.Errorf("内存组件未就绪")
	}

	var params struct {
		Rounds []int `json:"rounds"`
	}

	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %v", err)
	}

	if len(params.Rounds) == 0 {
		return "未指定任何轮次", nil
	}

	// 1. 获取所有原始消息
	allMsgs, _ := t.ChatMemory.ChatHistory.Messages(ctx)
	totalMsgs := len(allMsgs)

	// 2. 识别需要迁移和保留的消息
	// 逻辑：第 N 轮对应 index 为 (N-1)*2 和 (N-1)*2 + 1
	toStoreMap := make(map[int]bool)
	for _, r := range params.Rounds {
		humanIdx := (r - 1) * 2
		aiIdx := humanIdx + 1
		if humanIdx < totalMsgs {
			toStoreMap[humanIdx] = true
		}
		if aiIdx < totalMsgs {
			toStoreMap[aiIdx] = true
		}
	}

	var toStoreMsgs []llms.ChatMessage
	var remainingMsgs []llms.ChatMessage

	for i, msg := range allMsgs {
		if toStoreMap[i] {
			toStoreMsgs = append(toStoreMsgs, msg)
		} else {
			remainingMsgs = append(remainingMsgs, msg)
		}
	}

	// 3. 执行持久化
	if len(toStoreMsgs) > 0 {
		if err := t.Archiver.StoreMessages(ctx, toStoreMsgs); err != nil {
			return "", fmt.Errorf("持久化失败: %v", err)
		}
	}

	// 4. 重写内存（用 SetMessages 一次性替换，避免 Clear + N×AddMessage 的 N+1 次调用）
	if remainingMsgs == nil {
		remainingMsgs = []llms.ChatMessage{}
	}
	if err := t.ChatMemory.ChatHistory.SetMessages(ctx, remainingMsgs); err != nil {
		return "", fmt.Errorf("重写内存失败: %v", err)
	}

	return fmt.Sprintf("已将第 %v 轮对话移入长期记忆，内存已清理。", params.Rounds), nil
}
