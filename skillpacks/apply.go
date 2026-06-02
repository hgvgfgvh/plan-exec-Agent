package skillpacks

import (
	"AgentTest/capabilities"
	"AgentTest/config"
	"fmt"
	"strings"
)

// Apply 扫描 skill_packs、写入内存、合并 MCP server 定义，并注册外挂包目录提供者。
func Apply(cfg *config.App) error {
	mu.Lock()
	loaded = nil
	mu.Unlock()
	if cfg == nil || !cfg.Capabilities.SkillPacks.Enabled {
		return nil
	}
	packs, mcpLayers := ScanRoots(cfg)
	mu.Lock()
	loaded = packs
	mu.Unlock()

	mergePackMCPServers(cfg, packs, mcpLayers)

	capabilities.RegisterExternalPackCatalog(
		func() []capabilities.ExternalPackCatalog {
			ps := snapshotPacks()
			out := make([]capabilities.ExternalPackCatalog, len(ps))
			for i, p := range ps {
				out[i] = capabilities.ExternalPackCatalog{
					ID:          p.ID,
					Title:       p.Title,
					Description: p.Description,
					Summary:     p.BodySummary,
				}
			}
			return out
		},
		func(id string) (string, bool) {
			p, ok := packByID(id)
			if !ok {
				return "", false
			}
			return p.FullMarkdown, true
		},
	)

	fmt.Printf("[skill_packs] 已加载 %d 个外部能力包\n", len(packs))
	return nil
}

func mergePackMCPServers(cfg *config.App, packs []Pack, layers [][]config.MCPServerDef) {
	if cfg == nil {
		return
	}
	if !cfg.Capabilities.MCP.Enabled {
		for i, defs := range layers {
			if len(defs) == 0 {
				continue
			}
			id := ""
			if i < len(packs) {
				id = packs[i].ID
			}
			fmt.Printf("[skill_packs] 包 %s 内含 MCP 定义，但 capabilities.mcp.enabled=false，未连接这些 server\n", id)
		}
		return
	}
	used := make(map[string]bool)
	for _, s := range cfg.Capabilities.MCP.Servers {
		used[strings.ToLower(strings.TrimSpace(s.Name))] = true
	}
	for i, defs := range layers {
		packID := ""
		if i < len(packs) {
			packID = packs[i].ID
		}
		for _, def := range defs {
			if !def.Enabled {
				continue
			}
			d := def
			name := strings.TrimSpace(d.Name)
			if name == "" {
				name = "pack_" + packID
			}
			orig := name
			for suffix := 2; used[strings.ToLower(name)]; suffix++ {
				name = fmt.Sprintf("%s_%d", orig, suffix)
			}
			d.Name = name
			used[strings.ToLower(name)] = true
			cfg.Capabilities.MCP.Servers = append(cfg.Capabilities.MCP.Servers, d)
			fmt.Printf("[skill_packs] 自包 %s 合并 MCP server: %s\n", packID, d.Name)
		}
	}
}
