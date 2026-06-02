package capabilities

import (
	"AgentTest/behavior/skill"
	"AgentTest/config"
	"fmt"
	"sort"
	"strings"
)

// BuildPlanCapabilityOverview 为 PlanAgent 生成「仅能力体系名称」的概览（无入参/出参/Schema）。
func BuildPlanCapabilityOverview() string {
	cfg := config.TryGet()
	if cfg == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 能力体系概览（仅供步骤拆分；具体调用由执行 Agent 负责）\n\n")

	if sec := planMCPSection(cfg); sec != "" {
		b.WriteString(sec)
	}
	if sec := planBuiltinSection(); sec != "" {
		b.WriteString(sec)
	}
	if sec := planExternalSection(cfg); sec != "" {
		b.WriteString(sec)
	}
	b.WriteString("说明：编排时只引用上述名称；不要假设未列出的 MCP 工具或 SKILL。\n")
	return b.String()
}

func planMCPSection(cfg *config.App) string {
	if cfg == nil || !cfg.Capabilities.MCP.Enabled {
		return ""
	}
	servers := snapshotMCPServerCatalog()
	if len(servers) == 0 {
		return "## MCP\n（未连接）\n\n"
	}
	byServer := make(map[string][]string)
	for _, e := range snapshotMCPCatalog() {
		byServer[e.ServerName] = append(byServer[e.ServerName], e.PublicName)
	}
	var b strings.Builder
	b.WriteString("## MCP 服务\n")
	for _, s := range servers {
		b.WriteString(fmt.Sprintf("- 服务 `%s`", s.ServerName))
		if line := strings.TrimSpace(s.Brief); line != "" {
			b.WriteString("：")
			b.WriteString(truncateOneLine(line, 80))
		}
		b.WriteByte('\n')
		if names := byServer[s.ServerName]; len(names) > 0 {
			b.WriteString("  公开工具名：")
			for i, n := range names {
				if i > 0 {
					b.WriteString("、")
				}
				b.WriteString(n)
			}
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func planBuiltinSection() string {
	domains := skill.GlobalManager.CatalogSnapshot()
	if len(domains) == 0 {
		return ""
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Name < domains[j].Name })
	var b strings.Builder
	b.WriteString("## 内置 SKILL（执行时经 SetExecutorStep，此处仅列注册名）\n")
	for _, d := range domains {
		b.WriteString(fmt.Sprintf("- Domain `%s`\n", d.Name))
		abilities := append([]*skill.Ability(nil), d.Abilities...)
		sort.Slice(abilities, func(i, j int) bool { return abilities[i].Name < abilities[j].Name })
		for _, a := range abilities {
			for _, sk := range a.Skills {
				if sk == nil || strings.TrimSpace(sk.Name) == "" {
					continue
				}
				b.WriteString("  - ")
				b.WriteString(sk.Name)
				b.WriteByte('\n')
			}
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func planExternalSection(cfg *config.App) string {
	if cfg == nil {
		return ""
	}
	packs := snapshotExternalPacks()
	if len(packs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 外挂 SKILL 包\n")
	for _, p := range packs {
		b.WriteString(fmt.Sprintf("- 包 `%s`", p.ID))
		if t := strings.TrimSpace(p.Title); t != "" {
			b.WriteString(" / ")
			b.WriteString(truncateOneLine(t, 60))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}
