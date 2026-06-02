package prefrontalCortex

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/config"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// LLMExchangeLog 单次推送给 LLM 的 HTTP 交互（完整请求/响应，不做截断）。
type LLMExchangeLog struct {
	Timestamp  time.Time       `json:"timestamp"`
	TurnID     string          `json:"turn_id,omitempty"`
	Kind       string          `json:"kind"` // chat_completion | chat_completion_stream | legacy_chat
	Stream     bool            `json:"stream"`
	Model      string          `json:"model,omitempty"`
	Endpoint   string          `json:"endpoint,omitempty"`
	HTTPStatus int             `json:"http_status,omitempty"`
	Error      string          `json:"error,omitempty"`
	Request    json.RawMessage `json:"request"`
	Response   json.RawMessage `json:"response,omitempty"`
}

var llmExchangeSeq atomic.Uint64

// WriteLLMExchangeLog 将完整请求/响应写入配置的目录（capabilities.observability.llm_chat_log_dir）。
func WriteLLMExchangeLog(ctx context.Context, rec LLMExchangeLog) {
	dir := llmChatLogDir()
	if dir == "" {
		return
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}
	if ctx != nil {
		if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
			rec.TurnID = turnID
		}
	}
	if len(rec.Request) == 0 {
		rec.Request = json.RawMessage(`null`)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Printf("[llm_chat_log] 创建目录失败 %q: %v\n", dir, err)
		return
	}

	seq := llmExchangeSeq.Add(1)
	name := fmt.Sprintf("%s_%04d_%s.json",
		rec.Timestamp.Format("20060102T150405.000Z07"),
		seq%10000,
		sanitizeLogFileToken(rec.Kind),
	)
	path := filepath.Join(dir, name)

	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		fmt.Printf("[llm_chat_log] 序列化失败: %v\n", err)
		return
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fmt.Printf("[llm_chat_log] 写入失败 %q: %v\n", path, err)
		return
	}
	fmt.Printf("[llm_chat_log] 已写入 %s\n", path)
}

func llmChatLogDir() string {
	cfg := config.TryGet()
	if cfg == nil || !cfg.Capabilities.Observability.LLMChatLogEnabled {
		return ""
	}
	dir := strings.TrimSpace(cfg.Capabilities.Observability.LLMChatLogDir)
	if dir == "" {
		return ""
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(cfg.AbsRoot(), filepath.Clean(dir))
}

func sanitizeLogFileToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "exchange"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "exchange"
	}
	if len(out) > 48 {
		return out[:48]
	}
	return out
}

func sanitizeEndpointForLog(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	u.User = nil
	u.Fragment = ""
	return u.String()
}

func rawJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage(`null`)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(fmt.Sprintf(`{"marshal_error":%q}`, err.Error()))
	}
	return json.RawMessage(b)
}

func rawJSONBytes(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(`null`)
	}
	if json.Valid(b) {
		return json.RawMessage(b)
	}
	wrapped, _ := json.Marshal(map[string]string{"raw_text": string(b)})
	return json.RawMessage(wrapped)
}
