package soulhook

import (
	"context"
	"fmt"
	"strings"
)

// RetrieveTurnBeforeProcess OnTurnRetrieve：回合开始前同步 soul_retrieve，失败返回空串。
func (h *SoulMCPHook) RetrieveTurnBeforeProcess(ctx context.Context, userInput string) string {
	if h == nil || h.cfg == nil || !h.cfg.PlanSoulHook.Enabled {
		return ""
	}
	if h.provider == nil {
		return ""
	}
	hints := h.provider.RetrieveHints(ctx, userInput)
	if strings.TrimSpace(hints) == "" {
		return ""
	}
	fmt.Printf("[plan/soulhook] OnTurnRetrieve hints_len=%d\n", len([]rune(hints)))
	return hints
}

// CombineTurnHints 注入顺序：Soul（人格/议题）→ Memory（执行经验）→ 用户本轮输入。
func CombineTurnHints(userInput, soulHints, memoryHints string) string {
	userInput = strings.TrimSpace(userInput)
	var blocks []string
	if s := strings.TrimSpace(soulHints); s != "" {
		blocks = append(blocks, s)
	}
	if m := strings.TrimSpace(memoryHints); m != "" {
		blocks = append(blocks, m)
	}
	if len(blocks) == 0 {
		return userInput
	}
	joined := TurnHintPreamble + strings.Join(blocks, "\n\n")
	if userInput == "" {
		return joined
	}
	return joined + "\n\n---\n用户本轮输入:\n" + userInput
}
