package memoryhook

import (
	"AgentTest/plan/todolist"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var rePlanFooterID = regexp.MustCompile(`（编排\s+(.+?)\s+·`)

// TurnStoreInput Host 回合结束时的原始材料（portal 传入）。
type TurnStoreInput struct {
	TurnID         string
	UserInput      string
	AssistantReply string
	ProcessError   string
}

// StoreTurnAfterProcess OnTurnStore：异步 memory_store，失败仅打日志，不阻断门户。
func (h *MemoryMCPHook) StoreTurnAfterProcess(ctx context.Context, in TurnStoreInput) {
	if h == nil || h.cfg == nil || !h.cfg.PlanMemoryHook.Enabled || !h.StoreEnabled() {
		return
	}
	storer, ok := h.provider.(EpisodeStorer)
	if !ok {
		return
	}
	content := BuildEpisodeContent(in)
	if shouldSkipTurnStore(in.UserInput, content) {
		return
	}
	epIn := EpisodeStoreInput{
		TurnID:          strings.TrimSpace(in.TurnID),
		UserRequirement: strings.TrimSpace(in.UserInput),
		AssistantReply:  strings.TrimSpace(stripPlanMetaForStore(in.AssistantReply)),
		ProcessError:    strings.TrimSpace(in.ProcessError),
		PlanDocumentID:  parsePlanIDFromReply(in.AssistantReply),
	}
	go func() {
		bg := context.Background()
		if err := storer.StoreEpisode(bg, epIn, content); err != nil {
			fmt.Printf("[plan/memoryhook] memory_store 失败（已忽略，不阻断主流程）: %v\n", err)
			return
		}
		fmt.Printf("[plan/memoryhook] memory_store 已提交 turn=%s plan=%s\n", epIn.TurnID, epIn.PlanDocumentID)
	}()
	_ = ctx
}

// StoreEnabled 是否执行回合结束 store（与 Retrieve 路由独立；provider=noop 时无 Storer）。
func (h *MemoryMCPHook) StoreEnabled() bool {
	if h == nil || h.cfg == nil || !h.cfg.PlanMemoryHook.Enabled {
		return false
	}
	if h.cfg.PlanMemoryHook.StoreEnabled != nil {
		return *h.cfg.PlanMemoryHook.StoreEnabled
	}
	return true
}

// BuildEpisodeContent 将本轮材料序列化为单一 content 字符串（非 MCP 强制 Schema）。
func BuildEpisodeContent(in TurnStoreInput) string {
	var b strings.Builder
	turn := strings.TrimSpace(in.TurnID)
	planID := parsePlanIDFromReply(in.AssistantReply)
	b.WriteString("[source=agenttest-plan")
	if turn != "" {
		b.WriteString(" turn=" + turn)
	}
	if planID != "" {
		b.WriteString(" plan=" + planID)
	}
	b.WriteString("]\n\n")
	b.WriteString("## 用户诉求\n")
	b.WriteString(strings.TrimSpace(in.UserInput))
	b.WriteString("\n\n## 门户回复\n")
	b.WriteString(strings.TrimSpace(stripPlanMetaForStore(in.AssistantReply)))
	if err := strings.TrimSpace(in.ProcessError); err != "" {
		b.WriteString("\n\n## 处理错误\n")
		b.WriteString(err)
	}
	if doc := loadPlanDocument(planID); doc != nil {
		b.WriteString("\n\n## 计划终态 (TodoList)\n")
		b.WriteString(formatPlanDocument(doc))
	}
	return b.String()
}

func loadPlanDocument(planID string) *todolist.Document {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return nil
	}
	doc, err := todolist.Load(planID)
	if err != nil {
		return nil
	}
	return doc
}

func formatPlanDocument(doc *todolist.Document) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("id: %s\nstatus: %s\nexecution_mode: %s\nsummary: %s\n",
		doc.ID, doc.Status, doc.ExecutionMode, doc.Summary))
	for i, s := range doc.Steps {
		b.WriteString(fmt.Sprintf("step%d: [%s] %s tier=%d status=%s\n", i+1, s.ID, s.Title, s.Tier, s.Status))
		if rs := strings.TrimSpace(s.ResultSummary); rs != "" {
			b.WriteString("  result_summary: " + rs + "\n")
		}
		if len(s.Artifacts) > 0 {
			b.WriteString("  artifacts: " + strings.Join(s.Artifacts, ", ") + "\n")
		}
		if len(s.ToolsCalled) > 0 {
			b.WriteString("  tools_called: " + strings.Join(s.ToolsCalled, ", ") + "\n")
		}
	}
	if raw, err := json.MarshalIndent(doc, "", "  "); err == nil && len(raw) < 12000 {
		b.WriteString("\n--- document json ---\n")
		b.Write(raw)
	}
	return b.String()
}

func parsePlanIDFromReply(reply string) string {
	m := rePlanFooterID.FindStringSubmatch(reply)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func stripPlanMetaForStore(body string) string {
	const sep = "\n\n---\n（编排 "
	if i := strings.LastIndex(body, sep); i >= 0 {
		return strings.TrimSpace(body[:i])
	}
	return strings.TrimSpace(body)
}

func shouldSkipTurnStore(userInput, content string) bool {
	u := strings.TrimSpace(userInput)
	if len([]rune(u)) < 2 {
		return true
	}
	chitchat := map[string]struct{}{
		"你好": {}, "您好": {}, "hi": {}, "hello": {}, "谢谢": {}, "在吗": {},
	}
	if _, ok := chitchat[strings.ToLower(u)]; ok {
		return true
	}
	return len([]rune(strings.TrimSpace(content))) < 20
}
