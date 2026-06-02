package outputbus

// SSE 事件类型（空串表示兼容旧版整包消息）。
const (
	EventMessage = ""
	EventDelta   = "delta"
	EventFinal   = "final"
)

// PublishStreamDelta 向同一 message_id 追加一段增量文本（source 通常为「计划编排」）。
func PublishStreamDelta(source, turnID, messageID, chunk string) {
	if chunk == "" {
		return
	}
	publishEntry(Entry{
		Source:    source,
		Text:      chunk,
		TurnID:    turnID,
		MessageID: messageID,
		Event:     EventDelta,
	})
}

// PublishStreamFinal 标记流式消息结束；text 可为空或追加尾段（如 planMetaFooter）。
func PublishStreamFinal(source, turnID, messageID, text string) {
	publishEntry(Entry{
		Source:    source,
		Text:      text,
		TurnID:    turnID,
		MessageID: messageID,
		Event:     EventFinal,
	})
}

func publishEntry(e Entry) {
	startFanoutOnce()
	select {
	case broadcastCh <- e:
	default:
	}
}
