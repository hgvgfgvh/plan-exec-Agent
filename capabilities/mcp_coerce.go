package capabilities

import "strings"

// CoerceMCPArguments 修正模型常犯的 MCP 参数形状（在 CallTool 前调用）。
func CoerceMCPArguments(publicName string, args map[string]any) {
	if args == nil {
		return
	}
	lower := strings.ToLower(publicName)
	switch {
	case strings.Contains(lower, "send_email"), strings.Contains(lower, "send-email"):
		coerceResendSendEmailArgs(args)
	case strings.Contains(lower, "send_batch"), strings.Contains(lower, "send-batch"):
		coerceStringToStringSlice(args, "to", "cc", "bcc", "reply_to")
		if items, ok := args["emails"].([]any); ok {
			for _, it := range items {
				if m, ok := it.(map[string]any); ok {
					coerceResendSendEmailArgs(m)
				}
			}
		}
	}
}

func coerceResendSendEmailArgs(args map[string]any) {
	coerceStringToStringSlice(args, "to", "cc", "bcc", "reply_to")
}

func coerceStringToStringSlice(args map[string]any, keys ...string) {
	for _, k := range keys {
		v, ok := args[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			s := strings.TrimSpace(t)
			if s != "" {
				args[k] = []any{s}
			}
		case []any:
			// already array
		case []string:
			out := make([]any, len(t))
			for i, s := range t {
				out[i] = s
			}
			args[k] = out
		}
	}
}
