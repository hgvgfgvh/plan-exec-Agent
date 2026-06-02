package capabilities

import (
	"AgentTest/config"
	"strings"
)

func mcpServerAllowed(cfg *config.App, serverName string) bool {
	list := cfg.Capabilities.Security.AllowMCPServerNames
	if len(list) == 0 {
		return true
	}
	for _, n := range list {
		if strings.EqualFold(strings.TrimSpace(n), strings.TrimSpace(serverName)) {
			return true
		}
	}
	return false
}

func mcpToolDenied(cfg *config.App, publicToolName string) bool {
	lower := strings.ToLower(publicToolName)
	for _, s := range cfg.Capabilities.Security.DenyToolNameSubstrings {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}
