package capabilities

import (
	"AgentTest/config"
	"context"
	"log/slog"
	"os"
	"time"
	"unicode/utf8"
)

// LogNativeTool 记录本机（非 MCP）工具调用：slog + 可选审计文件。
func LogNativeTool(agentName, toolName string, d time.Duration, callErr error, resultLen int, paramJSON string) {
	cfg := config.TryGet()
	if cfg == nil || !cfg.Capabilities.Observability.Enabled || !cfg.Capabilities.Observability.NativeToolCalls {
		return
	}
	maxR := cfg.Capabilities.Observability.LogToolArgsMaxRunes
	snippet := truncateRunes(paramJSON, maxR)

	errStr := ""
	if callErr != nil {
		errStr = callErr.Error()
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("tool_call",
		"agent", agentName,
		"tool", toolName,
		"ms", d.Milliseconds(),
		"err", errStr,
		"result_len", resultLen,
		"params_snippet", snippet,
	)

	WriteAudit(cfg, map[string]any{
		"kind":        "native_tool",
		"agent":       agentName,
		"tool":        toolName,
		"duration_ms": d.Milliseconds(),
		"error":       errStr,
		"result_len":  resultLen,
		"params_snip": snippet,
	})
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

// ShutdownOnContext 在 ctx 取消时关闭 MCP（与 main defer 互补，保证交互退出即释放子进程）。
func ShutdownOnContext(ctx context.Context) {
	go func() {
		<-ctx.Done()
		Close()
	}()
}
