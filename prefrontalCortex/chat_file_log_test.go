package prefrontalCortex

import (
	"AgentTest/config"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLLMExchangeLog_fullPayload(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "app.yaml")
	cfgYAML := "root: " + filepath.ToSlash(dir) + "\ncapabilities:\n  observability:\n    llm_chat_log_enabled: true\n    llm_chat_log_dir: llm_chat\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	config.SetGlobal(cfg)

	req := chatHTTPPayload{
		Model: "test-model",
		Messages: []ChatAPIMessage{
			{Role: "user", Content: stringsRepeat("长文本", 2000)},
		},
	}
	respBody := []byte(`{"choices":[{"message":{"content":"ok"}}]}`)

	WriteLLMExchangeLog(context.Background(), LLMExchangeLog{
		Kind:       "chat_completion",
		Model:      "test-model",
		HTTPStatus: 200,
		Request:    rawJSON(req),
		Response:   rawJSONBytes(respBody),
	})

	logDir := filepath.Join(dir, "llm_chat")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 log file, got %d", len(entries))
	}
	b, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var rec LLMExchangeLog
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Request) < 500 {
		t.Fatalf("request too short, likely truncated: %d bytes", len(rec.Request))
	}
	var gotResp, wantResp map[string]any
	if err := json.Unmarshal(rec.Response, &gotResp); err != nil {
		t.Fatalf("parse logged response: %v", err)
	}
	if err := json.Unmarshal(respBody, &wantResp); err != nil {
		t.Fatal(err)
	}
	gotB, _ := json.Marshal(gotResp)
	wantB, _ := json.Marshal(wantResp)
	if string(gotB) != string(wantB) {
		t.Fatalf("response mismatch: got %s want %s", gotB, wantB)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
