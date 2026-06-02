package soulhook

import (
	"context"
	"fmt"
	"strings"
)

// StoreTurnAfterProcess OnTurnStore：异步 soul_store（仅 WebUI 对话），失败不阻断门户。
func (h *SoulMCPHook) StoreTurnAfterProcess(ctx context.Context, in WebUITurnInput) {
	if h == nil || h.cfg == nil || !h.cfg.PlanSoulHook.Enabled || !h.StoreEnabled() {
		return
	}
	storer, ok := h.provider.(DialogueStorer)
	if !ok {
		return
	}
	content := BuildWebUIDialogueContent(in)
	if shouldSkipSoulStore(in.UserInput, content) {
		return
	}
	epIn := DialogueStoreInput{
		TurnID:         strings.TrimSpace(in.TurnID),
		UserInput:      strings.TrimSpace(in.UserInput),
		AssistantReply: strings.TrimSpace(stripPortalMetaForSoul(in.AssistantReply)),
		ProcessError:   strings.TrimSpace(in.ProcessError),
	}
	go func() {
		bg := context.Background()
		if err := storer.StoreDialogue(bg, epIn, content); err != nil {
			fmt.Printf("[plan/soulhook] soul_store 失败（已忽略，不阻断主流程）: %v\n", err)
			return
		}
		fmt.Printf("[plan/soulhook] soul_store 已提交 turn=%s\n", epIn.TurnID)
	}()
	_ = ctx
}
