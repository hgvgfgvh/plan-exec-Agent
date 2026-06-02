package capabilities

import (
	"context"
	"testing"

	"github.com/tmc/langchaingo/tools"
)

type stubSuppressTool struct{ name string }

func (s stubSuppressTool) Name() string        { return s.name }
func (s stubSuppressTool) Description() string { return "mcp stub" }
func (s stubSuppressTool) Call(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (s stubSuppressTool) SuppressExecutorToolPrompt() bool { return true }

type stubPlainTool struct{ name string }

func (s stubPlainTool) Name() string        { return s.name }
func (s stubPlainTool) Description() string { return "plain" }
func (s stubPlainTool) Call(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestFilterToolMapForAPI_Progressive(t *testing.T) {
	mcpMu.Lock()
	mcpCatalog = []mcpToolCatalogEntry{
		{PublicName: "sqlite__list_tables", ServerName: "sqlite", FullDoc: "doc"},
		{PublicName: "resend__send_email", ServerName: "resend", FullDoc: "doc2"},
	}
	mcpMu.Unlock()
	t.Cleanup(func() {
		mcpMu.Lock()
		mcpCatalog = nil
		mcpMu.Unlock()
	})

	full := map[string]tools.Tool{
		"get_capability_details": getCapabilityDetailsTool{},
		"SetExecutorStep":        stubPlainTool{name: "SetExecutorStep"},
		"sqlite__list_tables":    stubSuppressTool{name: "sqlite__list_tables"},
		"resend__send_email":     stubSuppressTool{name: "resend__send_email"},
	}

	revealed := NewRevealedToolSet()
	filtered := FilterToolMapForAPI(full, revealed, true)
	if _, ok := filtered["sqlite__list_tables"]; ok {
		t.Fatal("unrevealed MCP should be hidden")
	}
	if _, ok := filtered["get_capability_details"]; !ok {
		t.Fatal("meta tool should stay")
	}

	revealed.RevealMCPTools([]string{"resend"})
	filtered = FilterToolMapForAPI(full, revealed, true)
	if _, ok := filtered["resend__send_email"]; !ok {
		t.Fatal("revealed MCP should appear")
	}
	if _, ok := filtered["sqlite__list_tables"]; ok {
		t.Fatal("other MCP should stay hidden")
	}
}

func TestRevealMCPTools_ByServer(t *testing.T) {
	mcpMu.Lock()
	mcpCatalog = []mcpToolCatalogEntry{
		{PublicName: "sqlite__list_tables", ServerName: "sqlite"},
		{PublicName: "sqlite__read_query", ServerName: "sqlite"},
	}
	mcpMu.Unlock()
	t.Cleanup(func() {
		mcpMu.Lock()
		mcpCatalog = nil
		mcpMu.Unlock()
	})

	r := NewRevealedToolSet()
	added := r.RevealMCPTools([]string{"sqlite"})
	if len(added) != 2 {
		t.Fatalf("server expand: got %v", added)
	}
}
