package turnjournal

import "time"

// Bundle 为一轮用户对话的统一日志（写入 WorkSpace/logs/turns/{turn_id}.json）。
type Bundle struct {
	TurnID    string    `json:"turn_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	UserInput string `json:"user_input"`
	PlanInput string `json:"plan_input,omitempty"` // Soul/Memory 注入后的 Plan 入参（与 user_input 可能不同）

	PathMode string `json:"path_mode"` // plan | behavior_fallback
	Status   string `json:"status"`    // ok | error

	Portal PortalSection `json:"portal"`
	Events []Event       `json:"events,omitempty"`
	Plan   *PlanSection  `json:"plan,omitempty"`

	ArtifactsIndex []ArtifactRef `json:"artifacts_index,omitempty"`

	ProcessError string `json:"process_error,omitempty"`
	LogPath      string `json:"log_path,omitempty"`
}

// PortalSection 门户层最终交付摘要。
type PortalSection struct {
	FinalReply        string `json:"final_reply,omitempty"`
	FinalReplyExcerpt string `json:"final_reply_excerpt,omitempty"`
	Streamed          bool   `json:"streamed,omitempty"`
	OutputSource      string `json:"output_source,omitempty"` // 计划编排 | 行为编排
}

// Event 为 outputbus 旁路采集到的一条门户可见输出。
type Event struct {
	At          time.Time `json:"at"`
	Source      string    `json:"source"`
	Text        string    `json:"text,omitempty"`
	TextExcerpt string    `json:"text_excerpt,omitempty"`
	Event       string    `json:"event,omitempty"` // delta | final
	MessageID   string    `json:"message_id,omitempty"`
}

// PlanSection 来自 TodoList 文档的结构化计划与步骤。
type PlanSection struct {
	DocumentID    string       `json:"document_id,omitempty"`
	DocumentPath  string       `json:"document_path,omitempty"`
	PlanStatus    string       `json:"plan_status,omitempty"`
	Summary       string       `json:"summary,omitempty"`
	ExecutionMode string       `json:"execution_mode,omitempty"`
	Steps         []StepRecord `json:"steps,omitempty"`
}

// StepRecord 单步执行记录（与 todolist.Step 对齐，便于后续 Run View 生成）。
type StepRecord struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	ResultSummary string   `json:"result_summary,omitempty"`
	ResultDetail  string   `json:"result_detail,omitempty"`
	ResultExcerpt string   `json:"result_excerpt,omitempty"`
	Artifacts     []string `json:"artifacts,omitempty"`
	ToolsCalled   []string `json:"tools_called,omitempty"`
}

// ArtifactRef 产物索引（路径相对或绝对，供后续 UI 链接解析）。
type ArtifactRef struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Type  string `json:"type,omitempty"` // image | file | dir | other
	Label string `json:"label,omitempty"`
}
