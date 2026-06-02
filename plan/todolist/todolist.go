// Package todolist 将 PlanAgent 的步骤计划持久化为 WorkSpace/ToDoList 下的 JSON 文档。
package todolist

import (
	"AgentTest/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// StepStatus 单步状态。
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
	StepBlocked   StepStatus = "blocked"
)

// PlanStatus 整份计划状态。
type PlanStatus string

const (
	PlanActive    PlanStatus = "active"
	PlanCompleted PlanStatus = "completed"
	PlanBlocked   PlanStatus = "blocked"
	PlanCancelled PlanStatus = "cancelled"
)

// FeedbackEntry 步骤上的反馈记录（结构化文本字段）。
type FeedbackEntry struct {
	At      time.Time `json:"at"`
	Phase   string    `json:"phase"` // create | dispatch | result | adjust | escalate
	Summary string    `json:"summary"`
}

// Step 计划中的一步。
type Step struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Instruction     string          `json:"instruction"`
	CapabilityHints []string        `json:"capability_hints,omitempty"`
	Tier            int             `json:"tier,omitempty"` // 1=轻验收 2=标准 3=重验收
	Status          StepStatus      `json:"status"`
	Attempts        int             `json:"attempts"`
	ResultSummary   string          `json:"result_summary,omitempty"`
	ResultDetail    string          `json:"result_detail,omitempty"` // 给用户看的完整正文
	Artifacts       []string        `json:"artifacts,omitempty"`
	ToolsCalled     []string        `json:"tools_called,omitempty"`
	Feedback        []FeedbackEntry `json:"feedback,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Document 一份独立需求的 TodoList。
type Document struct {
	ID              string     `json:"id"`
	UserRequirement string     `json:"user_requirement"`
	Summary         string     `json:"summary"`
	Status          PlanStatus `json:"status"`
	ExecutionMode   string     `json:"execution_mode,omitempty"` // exec | simple
	Steps           []Step     `json:"steps"`
	BlockedReason   string     `json:"blocked_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

var slugRe = regexp.MustCompile(`[^a-zA-Z0-9\p{Han}]+`)

// Dir 返回 ToDoList 根目录（不存在则创建）。
func Dir() (string, error) {
	cfg := config.Get()
	root := cfg.ResolvedPaths().Workspace
	dir := filepath.Join(root, "ToDoList")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// NewID 为一次用户诉求生成独立文档 ID。
func NewID(userRequirement string) string {
	slug := slugRe.ReplaceAllString(strings.TrimSpace(userRequirement), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "task"
	}
	runes := []rune(slug)
	if len(runes) > 24 {
		slug = string(runes[:24])
	}
	return fmt.Sprintf("%s-%d", slug, time.Now().Unix())
}

// Path 返回文档绝对路径。
func Path(id string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".json"), nil
}

// Save 写入或覆盖文档。
func Save(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("todolist: nil document")
	}
	doc.UpdatedAt = time.Now()
	path, err := Path(doc.ID)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load 读取文档。
func Load(id string) (*Document, error) {
	path, err := Path(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc Document
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// FormatForPrompt 将当前计划格式化为 PlanAgent 上下文（不含执行细节参数）。
func FormatForPrompt(doc *Document) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("计划ID: %s\n状态: %s\n需求摘要: %s\n", doc.ID, doc.Status, doc.Summary))
	if doc.BlockedReason != "" {
		b.WriteString("卡点: " + doc.BlockedReason + "\n")
	}
	b.WriteString("\n步骤:\n")
	for i, s := range doc.Steps {
		b.WriteString(fmt.Sprintf("%d. [%s] %s (id=%s, attempts=%d)\n", i+1, s.Status, s.Title, s.ID, s.Attempts))
		if len(s.CapabilityHints) > 0 {
			b.WriteString("   能力提示: " + strings.Join(s.CapabilityHints, ", ") + "\n")
		}
		if fb := lastFeedback(s); fb != "" {
			b.WriteString("   最近反馈: " + fb + "\n")
		}
	}
	return b.String()
}

func lastFeedback(s Step) string {
	if len(s.Feedback) == 0 {
		return ""
	}
	return s.Feedback[len(s.Feedback)-1].Summary
}

// StepResultText 返回该步骤最后一次 result 阶段摘要（无则空串）。
func StepResultText(s Step) string {
	for i := len(s.Feedback) - 1; i >= 0; i-- {
		if s.Feedback[i].Phase == "result" {
			return strings.TrimSpace(s.Feedback[i].Summary)
		}
	}
	return ""
}

// StepUserFacingText 返回该步骤应展示给用户的正文（优先已解析的 result_detail，否则 result_summary）。
func StepUserFacingText(s Step) string {
	if t := strings.TrimSpace(s.ResultDetail); t != "" {
		return t
	}
	if sum := strings.TrimSpace(s.ResultSummary); sum != "" {
		return sum
	}
	return StepResultText(s)
}

// CollectStepResults 按顺序收集已完成/跳过步骤的执行结果文本（供用户门面展示）。
func (d *Document) CollectStepResults() []string {
	if d == nil {
		return nil
	}
	var out []string
	for _, s := range d.Steps {
		switch s.Status {
		case StepCompleted, StepSkipped:
			if t := StepUserFacingText(s); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// LastNonEmptyResult 返回最后一个有 result 文本的步骤摘要（含 failed 步，便于报错展示）。
func (d *Document) LastNonEmptyResult() string {
	if d == nil {
		return ""
	}
	for i := len(d.Steps) - 1; i >= 0; i-- {
		if t := StepUserFacingText(d.Steps[i]); t != "" {
			return t
		}
	}
	return ""
}

// NextPending 返回下一个待执行步骤索引；无则 -1。
func (d *Document) NextPending() int {
	for i, s := range d.Steps {
		if s.Status == StepPending || s.Status == StepFailed {
			return i
		}
	}
	return -1
}

// AllTerminal 是否所有步骤已结束（完成/跳过/阻塞）。
func (d *Document) AllTerminal() bool {
	if len(d.Steps) == 0 {
		return true
	}
	for _, s := range d.Steps {
		switch s.Status {
		case StepCompleted, StepSkipped, StepBlocked:
			continue
		default:
			return false
		}
	}
	return true
}

// AppendFeedback 给指定步骤追加反馈并保存。
func (d *Document) AppendFeedback(stepIdx int, phase, summary string) {
	if stepIdx < 0 || stepIdx >= len(d.Steps) {
		return
	}
	s := &d.Steps[stepIdx]
	s.Feedback = append(s.Feedback, FeedbackEntry{
		At:      time.Now(),
		Phase:   phase,
		Summary: truncate(summary, 4000),
	})
	s.UpdatedAt = time.Now()
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
