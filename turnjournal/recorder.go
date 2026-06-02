package turnjournal

import (
	"strings"
	"sync"
	"time"
)

const (
	maxEventTextRunes = 8000
	maxFieldRunes     = 24000
	excerptRunes      = 500
)

var (
	activeMu sync.Mutex
	active   *recorder
)

type recorder struct {
	bundle Bundle
	mu     sync.Mutex

	// 流式「计划编排」按 message_id 合并，避免 events 膨胀。
	streamText map[string]string
}

// BeginInput 开启回合记录。
type BeginInput struct {
	TurnID    string
	UserInput string
	PlanInput string
	PathMode  string // plan | behavior_fallback
}

// Begin 标记新回合开始（与 runcontrol.BeginTurn 对齐调用）。
func Begin(in BeginInput) {
	if in.TurnID == "" {
		return
	}
	r := &recorder{
		bundle: Bundle{
			TurnID:    in.TurnID,
			StartedAt: time.Now(),
			UserInput: in.UserInput,
			PlanInput: in.PlanInput,
			PathMode:  in.PathMode,
			Status:    "ok",
		},
		streamText: make(map[string]string),
	}
	if strings.TrimSpace(in.UserInput) != "" {
		r.bundle.Events = append(r.bundle.Events, Event{
			At:          time.Now(),
			Source:      "user",
			Text:        truncateRunes(in.UserInput, maxEventTextRunes),
			TextExcerpt: excerpt(in.UserInput, excerptRunes),
		})
	}
	activeMu.Lock()
	active = r
	activeMu.Unlock()
}

// FinalizeInput 回合结束时的补充信息。
type FinalizeInput struct {
	TurnID       string
	Reply        string
	ProcessError string
	Streamed     bool
	OutputSource string // 计划编排 | 行为编排
}

// Finalize 落盘并清除活跃记录；失败仅打日志，不影响主链。
func Finalize(in FinalizeInput) {
	activeMu.Lock()
	r := active
	if r == nil || in.TurnID == "" || r.bundle.TurnID != in.TurnID {
		activeMu.Unlock()
		return
	}
	active = nil
	activeMu.Unlock()

	r.mu.Lock()
	r.bundle.EndedAt = time.Now()
	r.bundle.Portal = PortalSection{
		FinalReply:        truncateRunes(in.Reply, maxFieldRunes),
		FinalReplyExcerpt: excerpt(in.Reply, excerptRunes),
		Streamed:          in.Streamed,
		OutputSource:      in.OutputSource,
	}
	if in.ProcessError != "" {
		r.bundle.Status = "error"
		r.bundle.ProcessError = in.ProcessError
	}
	r.flushStreamEventsLocked()
	plan := buildPlanSection(in.Reply, r.bundle.StartedAt)
	if plan != nil {
		r.bundle.Plan = plan
		r.bundle.ArtifactsIndex = indexArtifacts(plan.Steps)
	}
	r.mu.Unlock()

	_ = writeBundle(&r.bundle)
}

func (r *recorder) flushStreamEventsLocked() {
	for msgID, text := range r.streamText {
		if strings.TrimSpace(text) == "" {
			continue
		}
		r.bundle.Events = append(r.bundle.Events, Event{
			At:          time.Now(),
			Source:      "计划编排",
			Text:        truncateRunes(text, maxEventTextRunes),
			TextExcerpt: excerpt(text, excerptRunes),
			Event:       "stream_merged",
			MessageID:   msgID,
		})
	}
	r.streamText = nil
}
