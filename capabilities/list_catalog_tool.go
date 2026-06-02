package capabilities

import (
	"context"
	"strings"
)

type listAgentCatalogTool struct{}

func (listAgentCatalogTool) Name() string { return "list_agent_capabilities" }

func (listAgentCatalogTool) Description() string {
	return `返回当前进程 AGENTS.md 第一层能力目录全文（与 system 文末注入内容一致）。用户问「有哪些能力/技能/MCP」时优先调用本工具，禁止自行编造未列出的能力。无参：Action Input: {}`
}

func (listAgentCatalogTool) Call(ctx context.Context, input string) (string, error) {
	_ = ctx
	_ = strings.TrimSpace(input)
	cat := strings.TrimSpace(BuildAgentCatalogMarkdown())
	if cat == "" {
		return "（当前无已挂载能力目录）", nil
	}
	return "# 能力目录（list_agent_capabilities）\n\n" + cat, nil
}

func init() {
	RegisterLangchainTools(listAgentCatalogTool{})
}
