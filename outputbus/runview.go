package outputbus

import "encoding/json"

// SourceRunView WebUI 运行视图抽屉（与计划编排主气泡独立）。
const SourceRunView = "运行视图"

// RunViewPayload SSE 载荷（JSON 字符串放在 Entry.Text）。
type RunViewPayload struct {
	TurnID  string `json:"turn_id"`
	Status  string `json:"status"` // pending | ready | failed
	HTMLURL string `json:"html_url,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PublishRunView 旁路通知前端：运行视图生成状态（不阻塞主链）。
func PublishRunView(turnID, status, htmlURL, errMsg string) {
	if turnID == "" {
		return
	}
	p := RunViewPayload{TurnID: turnID, Status: status, HTMLURL: htmlURL, Error: errMsg}
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	publishEntry(Entry{
		Source: SourceRunView,
		Text:   string(b),
		TurnID: turnID,
	})
}
