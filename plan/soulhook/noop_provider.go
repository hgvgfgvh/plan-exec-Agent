package soulhook

import "context"

// NoopProvider 未启用 Soul MCP 时的空实现。
type NoopProvider struct{}

func (NoopProvider) Name() string { return "noop" }

func (NoopProvider) RetrieveHints(context.Context, string) string { return "" }
