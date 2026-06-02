package attention

import (
	"context"
	"fmt"
	"sync"
)

type UpdateWorkingMemoryTool struct {
	// 基础 Prompt（不包含看板的部分）
	BasePrompt string
	// 当前看板内容
	currentBoard string
	// 指针注入：用于修改宿主 Agent 最终使用的系统提示词
	// 注意：这里注入的是一个指向 string 的指针，或者一个 Setter 方法
	TargetPromptPtr *string

	mu sync.Mutex
}

func (t *UpdateWorkingMemoryTool) Name() string {
	return "update_task_dashboard"
}

func (t *UpdateWorkingMemoryTool) Description() string {
	return `核心看板更新工具：记录最终目标、进度、关键变量。内容将固定在系统提示词中。直接输入你想要记录的文本内容。注意：此工具会覆盖旧看板！若要追加信息，请将旧看板的有效内容与新信息整合后一次性提交。`
}

func (t *UpdateWorkingMemoryTool) Call(ctx context.Context, input string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.TargetPromptPtr == nil {
		return "", fmt.Errorf("看板组件尚未注入目标 Prompt 指针")
	}

	// 1. 更新当前内容
	t.currentBoard = input

	// 2. 合成新的系统提示词
	// 使用特殊的分割符，确保 AI 能清晰分辨基础指令和动态看板
	updatedPrompt := t.BasePrompt + "\n\n[-----看板内容----]\n" + t.currentBoard

	// 3. 反向注入：修改宿主 Agent 的内存地址内容
	*t.TargetPromptPtr = updatedPrompt

	return "【系统消息】看板内容已固化，我会始终牢记。内容：" + input, nil
}
