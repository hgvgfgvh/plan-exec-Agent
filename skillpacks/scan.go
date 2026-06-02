package skillpacks

import (
	"AgentTest/config"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const skillFile = "SKILL.md"

func resolvePackRoot(cfg *config.App, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	return filepath.Join(cfg.AbsRoot(), filepath.Clean(rel))
}

// ScanRoots 扫描 roots 下的一层子目录，收集含 SKILL.md 的包及其可选 MCP 定义。
func ScanRoots(cfg *config.App) ([]Pack, [][]config.MCPServerDef) {
	var packs []Pack
	var mcpLayers [][]config.MCPServerDef

	for _, rel := range cfg.Capabilities.SkillPacks.Roots {
		root := resolvePackRoot(cfg, rel)
		if root == "" {
			continue
		}
		st, err := os.Stat(root)
		if err != nil {
			fmt.Printf("[skill_packs] 跳过根目录（不可访问）%q: %v\n", root, err)
			continue
		}
		if !st.IsDir() {
			fmt.Printf("[skill_packs] 根路径不是目录: %q\n", root)
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			fmt.Printf("[skill_packs] 列出目录失败 %q: %v\n", root, err)
			continue
		}
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			dir := filepath.Join(root, ent.Name())
			skillPath := filepath.Join(dir, skillFile)
			if _, err := os.Stat(skillPath); err != nil {
				alt := filepath.Join(dir, "skill.md")
				if _, err2 := os.Stat(alt); err2 == nil {
					skillPath = alt
				} else {
					continue
				}
			}
			raw, err := os.ReadFile(skillPath)
			if err != nil {
				fmt.Printf("[skill_packs] 读取 %q 失败: %v\n", skillPath, err)
				continue
			}
			title, desc, body := parseSkillFile(raw)
			id := sanitizePackID(ent.Name())
			summary := summarizeBody(body, 360)
			pack := Pack{
				ID:           id,
				Dir:          dir,
				Title:        title,
				Description:  desc,
				BodySummary:  summary,
				FullMarkdown: string(raw),
			}
			var layer []config.MCPServerDef
			for _, fname := range []string{"mcp.yaml", "mcp.yml"} {
				if b, err := os.ReadFile(filepath.Join(dir, fname)); err == nil {
					defs, err := ParseMCPServersFromYAML(b)
					if err != nil {
						fmt.Printf("[skill_packs] 包 %s %s 解析失败: %v\n", id, fname, err)
						continue
					}
					layer = append(layer, defs...)
				}
			}
			if b, err := os.ReadFile(filepath.Join(dir, "mcp.json")); err == nil {
				defs, err := ParseMCPServersFromCursorJSON(b)
				if err != nil {
					fmt.Printf("[skill_packs] 包 %s mcp.json 解析失败: %v\n", id, err)
				} else {
					layer = append(layer, defs...)
				}
			}
			for i := range layer {
				if !layer[i].Enabled {
					continue
				}
				if strings.TrimSpace(layer[i].WorkDir) == "" {
					layer[i].WorkDir = dir
				}
				if len(layer[i].Tags) == 0 {
					layer[i].Tags = []string{"skill_pack", id}
				}
			}
			packs = append(packs, pack)
			mcpLayers = append(mcpLayers, layer)
		}
	}
	return packs, mcpLayers
}

func sanitizePackID(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return "pack"
	}
	return s
}

func parseSkillFile(raw []byte) (title, description, body string) {
	fm, rest, ok := splitYAMLFrontmatter(raw)
	if ok {
		var meta struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		_ = yaml.Unmarshal(fm, &meta)
		if strings.TrimSpace(meta.Name) != "" {
			title = strings.TrimSpace(meta.Name)
		}
		description = strings.TrimSpace(meta.Description)
		body = strings.TrimSpace(string(rest))
	} else {
		body = strings.TrimSpace(string(raw))
	}
	if title == "" {
		title = firstHeadingOrLine(body)
	}
	if description == "" {
		description = firstNonEmptyLine(stripLeadingHashes(body))
	}
	return title, description, body
}

func splitYAMLFrontmatter(b []byte) (front []byte, rest []byte, ok bool) {
	if len(b) < 7 || !bytes.HasPrefix(b, []byte("---")) {
		return nil, b, false
	}
	i := 3
	if i < len(b) && b[i] == '\r' {
		i++
	}
	if i < len(b) && b[i] == '\n' {
		i++
	} else {
		return nil, b, false
	}
	closeIdx := bytes.Index(b[i:], []byte("\n---"))
	sepLen := len("\n---")
	if closeIdx < 0 {
		closeIdx = bytes.Index(b[i:], []byte("\r\n---"))
		sepLen = len("\r\n---")
	}
	if closeIdx < 0 {
		return nil, b, false
	}
	fm := b[i : i+closeIdx]
	j := i + closeIdx + sepLen
	for j < len(b) && (b[j] == '\r' || b[j] == '\n') {
		j++
	}
	return fm, b[j:], true
}

func firstHeadingOrLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "# "))
		}
		return truncateRunes(line, 120)
	}
	return "external_skill"
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 200)
		}
	}
	return ""
}

func stripLeadingHashes(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			lines[i] = strings.TrimSpace(strings.TrimLeft(t, "# "))
		}
		break
	}
	return strings.Join(lines, "\n")
}

func summarizeBody(body string, maxRunes int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	return truncateRunes(body, maxRunes)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if max <= 1 {
		return string(r[:1]) + "…"
	}
	return string(r[:max-1]) + "…"
}
