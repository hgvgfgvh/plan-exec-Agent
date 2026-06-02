package capabilities

import (
	"AgentTest/behavior/skill"
	"AgentTest/config"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// BuildAgentCatalogMarkdown 动态生成 AGENTS.md 风格的第一层能力目录（摘要），注入执行 Agent 的 system 上下文。
func BuildAgentCatalogMarkdown() string {
	cfg := config.TryGet()
	if cfg == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("当前进程已挂载能力索引（摘要）。完整 Schema / SKILL 全文 / 内置参数：工具 `get_capability_details`（可一次指定多项）。\n\n")

	hasAny := false
	if sec := buildMCPCatalogSection(cfg); sec != "" {
		b.WriteString(sec)
		hasAny = true
	}
	if sec := buildBuiltinSkillCatalogSection(); sec != "" {
		b.WriteString(sec)
		hasAny = true
	}
	if sec := buildExternalPackCatalogSection(cfg); sec != "" {
		b.WriteString(sec)
		hasAny = true
	}
	b.WriteString(buildCatalogUsageSection())

	if !hasAny {
		return ""
	}
	return b.String()
}

func buildCatalogUsageSection() string {
	return "## 第二层\n\n" +
		"工具 `get_capability_details`（仅查 Schema/SKILL 全文，**不能代替真实执行**）：\n" +
		"- **mcp_tools**：MCP **server 名**（展开该服务全部 Schema）或 **公开工具名**\n" +
		"- **external_skills**：外挂包 id\n" +
		"- **builtin_skills**：内置技能注册名\n" +
		"执行任务须：`Action: <MCP公开名>` 或 `SetExecutorStep`；禁止编造未在 AGENTS.md 列出的技能。\n\n"
}

func buildMCPCatalogSection(cfg *config.App) string {
	if cfg == nil || !cfg.Capabilities.MCP.Enabled {
		return ""
	}
	servers := snapshotMCPServerCatalog()
	if len(servers) == 0 {
		return "## MCP 服务\n\n（未连接或未注册 MCP；检查 capabilities.mcp 与启动日志。）\n\n"
	}

	byServer := make(map[string][]string)
	for _, e := range snapshotMCPCatalog() {
		byServer[e.ServerName] = append(byServer[e.ServerName], e.PublicName)
	}
	for srv := range byServer {
		sort.Strings(byServer[srv])
	}

	var b strings.Builder
	b.WriteString("## MCP 服务（Action 使用下列公开名；完整 Schema 用第二层 `get_capability_details`）\n\n")
	for _, s := range servers {
		b.WriteString("### ")
		b.WriteString(s.ServerName)
		b.WriteByte('\n')
		if line := strings.TrimSpace(s.Brief); line != "" {
			b.WriteString("- 功能：")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteString("- 工具数：")
		b.WriteString(fmt.Sprintf("%d", s.ToolCount))
		b.WriteByte('\n')
		if names := byServer[s.ServerName]; len(names) > 0 {
			b.WriteString("- 公开名：")
			for i, n := range names {
				if i > 0 {
					b.WriteString("、")
				}
				b.WriteString("`")
				b.WriteString(n)
				b.WriteString("`")
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func buildBuiltinSkillCatalogSection() string {
	domains := skill.GlobalManager.CatalogSnapshot()
	if len(domains) == 0 {
		return ""
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Name < domains[j].Name })

	var b strings.Builder
	b.WriteString("## 内置技能（须 SetExecutorStep；非顶格 Action 名）\n\n")
	for _, d := range domains {
		b.WriteString("### Domain: ")
		b.WriteString(d.Name)
		if desc := strings.TrimSpace(d.Description); desc != "" {
			b.WriteString(" — ")
			b.WriteString(truncateOneLine(desc, 120))
		}
		b.WriteByte('\n')
		abilities := append([]*skill.Ability(nil), d.Abilities...)
		sort.Slice(abilities, func(i, j int) bool { return abilities[i].Name < abilities[j].Name })
		domainSkillCount := 0
		for _, a := range abilities {
			b.WriteString("- **Ability** `")
			b.WriteString(a.Name)
			b.WriteString("`")
			if desc := strings.TrimSpace(a.Description); desc != "" {
				b.WriteString(" — ")
				b.WriteString(truncateOneLine(desc, 100))
			}
			b.WriteByte('\n')
			skills := append([]*skill.SkillInfo(nil), a.Skills...)
			sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
			for _, s := range skills {
				if s.Instance == nil {
					continue
				}
				domainSkillCount++
				b.WriteString("  - `")
				b.WriteString(s.Name)
				b.WriteString("`")
				if desc := strings.TrimSpace(s.Description); desc != "" {
					b.WriteString(" — ")
					b.WriteString(truncateOneLine(desc, 100))
				}
				b.WriteByte('\n')
			}
		}
		if domainSkillCount == 0 {
			b.WriteString("- （本域暂无已启用技能）\n")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func buildExternalPackCatalogSection(cfg *config.App) string {
	if cfg == nil || !cfg.Capabilities.SkillPacks.Enabled {
		return ""
	}
	packs := snapshotExternalPacks()
	if len(packs) == 0 {
		return "## 外挂能力包（SKILL.md）\n\n（已启用 skill_packs，但 roots 下无含 SKILL.md 的包。）\n\n"
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].ID < packs[j].ID })

	var b strings.Builder
	b.WriteString("## 外挂能力包（SKILL.md）\n\n")
	for _, p := range packs {
		b.WriteString("- **id** `")
		b.WriteString(p.ID)
		b.WriteString("`")
		if t := strings.TrimSpace(p.Title); t != "" {
			b.WriteString(" | title: ")
			b.WriteString(t)
		}
		if d := strings.TrimSpace(p.Description); d != "" {
			b.WriteString(" | ")
			b.WriteString(truncateOneLine(d, 80))
		}
		if s := strings.TrimSpace(p.Summary); s != "" {
			b.WriteString("\n  - summary: ")
			b.WriteString(truncateOneLine(s, 200))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func truncateOneLine(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	if maxRunes <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxRunes]) + "…"
}

// snapshotMCPServerCatalog 返回 MCP 服务级目录快照（第一层摘要）。
func snapshotMCPServerCatalog() []mcpServerCatalogEntry {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	if len(mcpServerCatalog) == 0 {
		return nil
	}
	out := make([]mcpServerCatalogEntry, len(mcpServerCatalog))
	copy(out, mcpServerCatalog)
	return out
}

// snapshotMCPCatalog 返回 MCP 工具目录快照（供目录与详情工具使用）。
func snapshotMCPCatalog() []mcpToolCatalogEntry {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	if len(mcpCatalog) == 0 {
		return nil
	}
	out := make([]mcpToolCatalogEntry, len(mcpCatalog))
	copy(out, mcpCatalog)
	return out
}

func mcpCatalogFullDoc(publicName string) (string, bool) {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	for _, e := range mcpCatalog {
		if e.PublicName == publicName {
			return e.FullDoc, true
		}
	}
	return "", false
}

// mcpCatalogDocsByServer 第二层：按 MCP server 名展开该服务下全部工具的 FullDoc。
func mcpCatalogDocsByServer(serverName string) (string, bool) {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return "", false
	}
	mcpMu.Lock()
	defer mcpMu.Unlock()
	var b strings.Builder
	n := 0
	for _, e := range mcpCatalog {
		if e.ServerName != serverName {
			continue
		}
		n++
		if n > 1 {
			b.WriteString("\n---\n\n")
		}
		b.WriteString("### `")
		b.WriteString(e.PublicName)
		b.WriteString("`\n\n")
		b.WriteString(e.FullDoc)
	}
	if n == 0 {
		return "", false
	}
	header := fmt.Sprintf("**MCP server `%s`**（共 %d 个工具；Action 使用下列公开名）\n\n", serverName, n)
	return header + b.String(), true
}

// resolveMCPDetailDoc 按公开工具名或 server 名解析第二层文档。
func resolveMCPDetailDoc(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	if doc, ok := mcpCatalogFullDoc(name); ok {
		return doc, true
	}
	return mcpCatalogDocsByServer(name)
}

func firstLineSummary(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.IndexAny(text, "\n\r"); idx >= 0 {
		text = text[:idx]
	}
	return truncateOneLine(text, maxRunes)
}

// executorToAttachKey 将 CustomExecutor.AgentName 映射到 config capabilities.attach_to 的 key。
func executorToAttachKey(executorName string) string {
	switch strings.ToLower(strings.TrimSpace(executorName)) {
	case "behavioragent":
		return "behaviorAgent"
	case "execsimpleagent":
		return "execSimpleAgent"
	case "affectiveinteractiveagent":
		return "interactiveAgent"
	case "baseagent":
		return "baseAgent"
	default:
		return executorName
	}
}

// CatalogAttachedToExecutor 该 Executor 是否应注入能力目录（按 attach_to 判定）。
func CatalogAttachedToExecutor(executorName string) bool {
	cfg := config.TryGet()
	if cfg == nil {
		return false
	}
	return shouldAttach(cfg, executorToAttachKey(executorName))
}

// FormatCatalogForExecutor 为执行类 Agent 在 system 末尾拼接 AGENTS.md 第一层目录。
func FormatCatalogForExecutor(executorAgentName string) string {
	if !CatalogAttachedToExecutor(executorAgentName) {
		return ""
	}
	cat := strings.TrimSpace(BuildAgentCatalogMarkdown())
	if cat == "" {
		return ""
	}
	return fmt.Sprintf("\n\n# AGENTS.md（运行时能力目录·第一层）\n\n%s\n", cat)
}
