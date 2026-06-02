package memoryhook

// Experience 为 Plan 路由 Exec-Simple 时使用的结构化经验（通常来自 Memory MCP retrieve）。
type Experience struct {
	Matched    bool
	Confidence float64
	Summary    string
	PathHint   string
}
