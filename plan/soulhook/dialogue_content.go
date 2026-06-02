package soulhook

import "strings"

// WebUITurnInput 门户回合结束时的 WebUI 对话材料（不含 Memory 用的 TodoList/计划终态）。
type WebUITurnInput struct {
	TurnID         string
	UserInput      string
	AssistantReply string
	ProcessError   string
	Channel        string // web | mobile | desktop | embedded；空则 web
}

// BuildWebUIDialogueContent 序列化为 soul_store 的 content（与 memory_store 的 episode 材料分离）。
func BuildWebUIDialogueContent(in WebUITurnInput) string {
	var b strings.Builder
	turn := strings.TrimSpace(in.TurnID)
	ch := strings.TrimSpace(in.Channel)
	if ch == "" {
		ch = "web"
	}
	b.WriteString("[source=agenttest-")
	b.WriteString(ch)
	if turn != "" {
		b.WriteString(" turn=" + turn)
	}
	b.WriteString("]\n\n")
	b.WriteString("## 用户（")
	b.WriteString(ch)
	b.WriteString("）\n")
	b.WriteString(strings.TrimSpace(in.UserInput))
	b.WriteString("\n\n## 助手（")
	b.WriteString(ch)
	b.WriteString("）\n")
	b.WriteString(strings.TrimSpace(stripPortalMetaForSoul(in.AssistantReply)))
	if err := strings.TrimSpace(in.ProcessError); err != "" {
		b.WriteString("\n\n## 处理错误\n")
		b.WriteString(err)
	}
	return b.String()
}

func stripPortalMetaForSoul(body string) string {
	const sep = "\n\n---\n（编排 "
	if i := strings.LastIndex(body, sep); i >= 0 {
		return strings.TrimSpace(body[:i])
	}
	return strings.TrimSpace(body)
}

func shouldSkipSoulStore(userInput, content string) bool {
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
	return len([]rune(strings.TrimSpace(content))) < 16
}
