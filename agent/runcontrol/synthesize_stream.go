package runcontrol

import (
	"context"
	"sync"
)

type synthesizeStreamKey struct{}

type synthesizeStreamState struct {
	streamed  bool
	messageID string
}

var (
	synthesizeStreamMu   sync.Mutex
	synthesizeStreamTurn = map[string]string{} // turnID -> messageID（跨 PlanAgent/portal 可见）
)

// MarkSynthesizeStreamActive 标记本回合正在/已经流式推送交付助手（须用 TurnMeta 中的 turnID）。
func MarkSynthesizeStreamActive(turnID, messageID string) {
	if turnID == "" || messageID == "" {
		return
	}
	synthesizeStreamMu.Lock()
	synthesizeStreamTurn[turnID] = messageID
	synthesizeStreamMu.Unlock()
}

// ClearSynthesizeStream 清除回合流式标记（流式失败回退整包、或门户已处理完后调用）。
func ClearSynthesizeStream(turnID string) {
	if turnID == "" {
		return
	}
	synthesizeStreamMu.Lock()
	delete(synthesizeStreamTurn, turnID)
	synthesizeStreamMu.Unlock()
}

// BeginSynthesizeStream 同时写入 context 与 turn 表；turnID 优先用显式传入（避免仅改局部 ctx 导致门户读不到）。
func BeginSynthesizeStream(ctx context.Context, turnID, messageID string) context.Context {
	if turnID == "" {
		turnID, _ = TurnMetaFromContext(ctx)
	}
	MarkSynthesizeStreamActive(turnID, messageID)
	if ctx == nil {
		ctx = context.Background()
	}
	st := &synthesizeStreamState{streamed: true, messageID: messageID}
	return context.WithValue(ctx, synthesizeStreamKey{}, st)
}

// SynthesizeStreamed 本回合是否已通过流式推送交付助手正文（portal 应跳过整包「计划编排」）。
func SynthesizeStreamed(ctx context.Context) bool {
	if turnID, _ := TurnMetaFromContext(ctx); turnID != "" {
		synthesizeStreamMu.Lock()
		_, ok := synthesizeStreamTurn[turnID]
		synthesizeStreamMu.Unlock()
		if ok {
			return true
		}
	}
	st, ok := ctx.Value(synthesizeStreamKey{}).(*synthesizeStreamState)
	return ok && st != nil && st.streamed
}

// SynthesizeStreamMessageID 流式气泡 ID（空表示未流式）。
func SynthesizeStreamMessageID(ctx context.Context) string {
	if turnID, _ := TurnMetaFromContext(ctx); turnID != "" {
		synthesizeStreamMu.Lock()
		mid := synthesizeStreamTurn[turnID]
		synthesizeStreamMu.Unlock()
		if mid != "" {
			return mid
		}
	}
	st, ok := ctx.Value(synthesizeStreamKey{}).(*synthesizeStreamState)
	if !ok || st == nil {
		return ""
	}
	return st.messageID
}
