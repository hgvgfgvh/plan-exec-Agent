package capabilities

import (
	"AgentTest/behavior/skill"
	"strings"
)

// ExistsCapability 判断名称是否为已知的 MCP 公开名或已注册内置 SKILL。
func ExistsCapability(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.Contains(name, "__") {
		for _, e := range snapshotMCPCatalog() {
			if e.PublicName == name {
				return true
			}
		}
		return false
	}
	return skill.GlobalManager.HasRegisteredSkill(name)
}

// SanitizeCapabilityHints 移除非法能力名，返回清洗后的列表与被移除项。
func SanitizeCapabilityHints(hints []string) (cleaned, removed []string) {
	for _, h := range hints {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ExistsCapability(h) {
			cleaned = append(cleaned, h)
		} else {
			removed = append(removed, h)
		}
	}
	return cleaned, removed
}
