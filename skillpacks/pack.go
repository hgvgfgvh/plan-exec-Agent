package skillpacks

// Pack 表示 skill_packs 根目录下一层子目录中的一套外部能力（SKILL.md + 可选 MCP 配置）。
type Pack struct {
	ID           string
	Dir          string
	Title        string
	Description  string
	BodySummary  string
	FullMarkdown string
}
