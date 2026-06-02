package turnjournal

import (
	"AgentTest/outputbus"
	"context"
	"strings"
	"time"
)

var recordSources = map[string]bool{
	"user": true,
	"计划编排": true,
	"计划进度": true,
	"行为编排": true,
	"系统":   true,
	"系统异常": true,
}

// StartSubscriber 订阅 outputbus，将门户可见输出旁路写入当前活跃回合（不改变广播逻辑）。
func StartSubscriber(ctx context.Context) {
	ch, cancel := outputbus.Subscribe(256)
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				observeOutput(e)
			}
		}
	}()
}

func observeOutput(e outputbus.Entry) {
	src := strings.TrimSpace(e.Source)
	if !recordSources[src] {
		return
	}

	activeMu.Lock()
	r := active
	activeMu.Unlock()
	if r == nil {
		return
	}

	tid := strings.TrimSpace(e.TurnID)
	if tid != "" && tid != r.bundle.TurnID {
		return
	}

	ev := strings.TrimSpace(e.Event)
	text := e.Text

	r.mu.Lock()
	defer r.mu.Unlock()

	// 流式计划编排：按 message_id 合并，finalize 时再写入 events。
	if src == "计划编排" && (ev == "delta" || ev == "final") {
		mid := strings.TrimSpace(e.MessageID)
		if mid == "" {
			mid = "default"
		}
		if text != "" {
			r.streamText[mid] += text
		}
		if ev == "final" {
			merged := r.streamText[mid]
			delete(r.streamText, mid)
			if strings.TrimSpace(merged) != "" {
				r.bundle.Events = append(r.bundle.Events, Event{
					At:          time.Now(),
					Source:      src,
					Text:        truncateRunes(merged, maxEventTextRunes),
					TextExcerpt: excerpt(merged, excerptRunes),
					Event:       "final",
					MessageID:   mid,
				})
			}
		}
		return
	}

	if strings.TrimSpace(text) == "" {
		return
	}
	r.bundle.Events = append(r.bundle.Events, Event{
		At:          time.Now(),
		Source:      src,
		Event:       ev,
		MessageID:   strings.TrimSpace(e.MessageID),
		Text:        truncateRunes(text, maxEventTextRunes),
		TextExcerpt: excerpt(text, excerptRunes),
	})
}
