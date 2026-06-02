package memoryhook

import (
	"context"
	"fmt"
	"strings"
)

// RetrieveTurnBeforeProcess 宪法 OnTurnRetrieve：回合开始前同步 retrieve，失败返回空串。
func (h *MemoryMCPHook) RetrieveTurnBeforeProcess(ctx context.Context, userInput string) string {
	if h == nil || h.cfg == nil || !h.cfg.PlanMemoryHook.Enabled {
		return ""
	}
	p, ok := h.provider.(*MCPProvider)
	if !ok {
		return ""
	}
	hints := p.RetrieveTurnHints(ctx, userInput)
	if strings.TrimSpace(hints) == "" {
		return ""
	}
	fmt.Printf("[plan/memoryhook] OnTurnRetrieve hints_len=%d\n", len([]rune(hints)))
	return hints
}

// InjectTurnHints 将跨会话参考拼到用户输入前（Host 第一层 + 第三层）。
func InjectTurnHints(userInput, hints string) string {
	hints = strings.TrimSpace(hints)
	userInput = strings.TrimSpace(userInput)
	if hints == "" {
		return userInput
	}
	if userInput == "" {
		return hints
	}
	return hints + "\n\n---\n用户本轮输入:\n" + userInput
}
