package capabilities

import (
	"strings"
	"testing"
)

func TestResolveMCPDetailDoc(t *testing.T) {
	mcpMu.Lock()
	mcpCatalog = []mcpToolCatalogEntry{
		{PublicName: "filesystem__read_text_file", ServerName: "filesystem", FullDoc: "doc-read"},
		{PublicName: "filesystem__write_file", ServerName: "filesystem", FullDoc: "doc-write"},
		{PublicName: "resend__send_email", ServerName: "resend", FullDoc: "doc-send"},
	}
	mcpServerCatalog = []mcpServerCatalogEntry{
		{ServerName: "filesystem", Brief: "files", ToolCount: 2},
	}
	mcpMu.Unlock()
	t.Cleanup(func() {
		mcpMu.Lock()
		mcpCatalog = nil
		mcpServerCatalog = nil
		mcpMu.Unlock()
	})

	doc, ok := resolveMCPDetailDoc("filesystem__read_text_file")
	if !ok || doc != "doc-read" {
		t.Fatalf("tool name: got ok=%v doc=%q", ok, doc)
	}

	doc, ok = resolveMCPDetailDoc("filesystem")
	if !ok || !strings.Contains(doc, "doc-read") || !strings.Contains(doc, "doc-write") {
		t.Fatalf("server expand: got ok=%v doc=%q", ok, doc)
	}

	_, ok = resolveMCPDetailDoc("unknown")
	if ok {
		t.Fatal("expected miss for unknown")
	}
}
