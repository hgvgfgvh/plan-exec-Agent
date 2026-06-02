package memoryhook

import (
	"AgentTest/config"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	RegisterProvider("mcp", newMCPProvider)
}

// MCPProvider 通过 stdio 连接 AgentTestMemoryMCP（memory_store / memory_retrieve），字符串协议。
type MCPProvider struct {
	session *mcpsdk.ClientSession
}

func newMCPProvider(cfg *config.App) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("memoryhook mcp: nil config")
	}
	cmd := strings.TrimSpace(cfg.PlanMemoryHook.MCPCommand)
	if cmd == "" {
		return nil, fmt.Errorf("memoryhook mcp: plan_memory_hook.mcp_command 未配置")
	}
	cmd = resolveCommandPath(cfg, cmd)
	engineKind := strings.TrimSpace(cfg.PlanMemoryHook.MCPEngine)
	if engineKind == "" {
		engineKind = "factworld"
	}
	args := []string{"-engine", engineKind}
	ctx := context.Background()
	execCmd := exec.CommandContext(ctx, cmd, args...)
	execCmd.Env = os.Environ()
	for k, v := range cfg.PlanMemoryHook.MCPEnv {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}
	transport := &mcpsdk.CommandTransport{Command: execCmd}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "AgentTest-memoryhook", Version: "0.1"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("memoryhook mcp connect: %w", err)
	}
	return &MCPProvider{session: sess}, nil
}

func (p *MCPProvider) Name() string { return "mcp" }

func (p *MCPProvider) Retrieve(ctx context.Context, req RetrieveRequest) (Experience, error) {
	if p == nil || p.session == nil {
		return Experience{}, fmt.Errorf("memoryhook mcp: no session")
	}
	contextStr := buildRetrieveContext(req)
	res, err := p.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_retrieve",
		Arguments: map[string]any{
			"context":    contextStr,
			"query_hint": req.Document.UserRequirement,
		},
	})
	if err != nil {
		return Experience{}, err
	}
	raw := extractMCPText(res)
	hintsBody := extractHintsField(raw)
	exp := parseExperienceFromHints(hintsBody)
	fmt.Printf("[plan/memoryhook] memory_retrieve matched=%v confidence=%.2f hints_len=%d\n",
		exp.Matched, exp.Confidence, len([]rune(hintsBody)))
	return exp, nil
}

func buildRetrieveContext(req RetrieveRequest) string {
	var b strings.Builder
	b.WriteString("用户诉求: ")
	b.WriteString(req.UserRequirement)
	b.WriteString("\n")
	if req.Document != nil {
		if s := strings.TrimSpace(req.Document.Summary); s != "" {
			b.WriteString("计划摘要: ")
			b.WriteString(s)
			b.WriteString("\n")
		}
		b.WriteString("计划状态: ")
		b.WriteString(string(req.Document.Status))
		b.WriteString("\n")
	}
	return b.String()
}

func extractMCPText(res *mcpsdk.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcpsdk.TextContent); ok && strings.TrimSpace(t.Text) != "" {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

var (
	reMatchTag  = regexp.MustCompile(`\[exec_simple_match=(yes|no)\s*(?:confidence=([0-9.]+))?\]`)
	reConfHints = regexp.MustCompile(`confidence[=:]\s*([0-9.]+)`)
)

// extractHintsField 从 memory_retrieve 的 JSON 文本中取出 hints；非 JSON 则原样返回。
func extractHintsField(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var payload struct {
		Hints string `json:"hints"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err == nil && strings.TrimSpace(payload.Hints) != "" {
		return payload.Hints
	}
	return raw
}

func parseExperienceFromHints(hints string) Experience {
	exp := Experience{Summary: strings.TrimSpace(hints)}
	if match, conf, ok := parseMemoryRouteBlock(hints); ok {
		exp.Matched = strings.EqualFold(match, "yes")
		exp.Confidence = conf
	}
	if m := reMatchTag.FindStringSubmatch(hints); len(m) >= 2 {
		exp.Matched = strings.EqualFold(m[1], "yes")
		if len(m) >= 3 && m[2] != "" {
			fmt.Sscanf(m[2], "%f", &exp.Confidence)
		}
	}
	if exp.Confidence <= 0 && exp.Matched {
		exp.Confidence = 0.88
	}
	if !exp.Matched {
		if c := reConfHints.FindStringSubmatch(hints); len(c) >= 2 {
			fmt.Sscanf(c[1], "%f", &exp.Confidence)
		}
	}
	if idx := strings.Index(hints, "tools:"); idx >= 0 {
		line := hints[idx+len("tools:"):]
		if nl := strings.Index(line, "\n"); nl >= 0 {
			line = line[:nl]
		}
		exp.PathHint = strings.TrimSpace(line)
	}
	if idx := strings.Index(hints, "路径提示:"); idx >= 0 {
		exp.PathHint = strings.TrimSpace(hints[idx+len("路径提示:"):])
		if nl := strings.Index(exp.PathHint, "\n"); nl >= 0 {
			exp.PathHint = strings.TrimSpace(exp.PathHint[:nl])
		}
	}
	if exp.Summary != "" && len([]rune(exp.Summary)) > 800 {
		exp.Summary = string([]rune(exp.Summary)[:800]) + "…"
	}
	return exp
}

func resolveCommandPath(cfg *config.App, command string) string {
	command = strings.TrimSpace(command)
	if command == "" || filepath.IsAbs(command) {
		return command
	}
	if !strings.ContainsAny(command, `/\`) {
		return command
	}
	return filepath.Join(cfg.AbsRoot(), filepath.Clean(filepath.FromSlash(command)))
}

// StoreEpisode 实现 EpisodeStorer，调用 Memory MCP memory_store。
func (p *MCPProvider) StoreEpisode(ctx context.Context, in EpisodeStoreInput, content string) error {
	if p == nil || p.session == nil {
		return fmt.Errorf("memoryhook mcp: no session")
	}
	source := "agenttest-plan"
	kind := "episode"
	correlation := in.TurnID
	if correlation == "" {
		correlation = in.PlanDocumentID
	}
	_, err := p.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_store",
		Arguments: map[string]any{
			"content":        content,
			"source":         source,
			"kind":           kind,
			"correlation_id": correlation,
		},
	})
	return err
}

var _ EpisodeStorer = (*MCPProvider)(nil)

var _ Provider = (*MCPProvider)(nil)

const memoryRouteMarker = "---memory-route---"

func parseMemoryRouteBlock(hints string) (match string, confidence float64, ok bool) {
	idx := strings.Index(hints, memoryRouteMarker)
	if idx < 0 {
		return "", 0, false
	}
	rest := hints[idx+len(memoryRouteMarker):]
	start := strings.Index(rest, "{")
	end := strings.LastIndex(rest, "}")
	if start < 0 || end <= start {
		return "", 0, false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rest[start:end+1]), &m); err != nil {
		return "", 0, false
	}
	if v, _ := m["exec_simple_match"].(string); v != "" {
		match = v
	}
	switch c := m["confidence"].(type) {
	case float64:
		confidence = c
	}
	return match, confidence, true
}

// RetrieveTurnHints OnTurnRetrieve：回合开始前拉取跨会话参考（不阻断主流程）。
func (p *MCPProvider) RetrieveTurnHints(ctx context.Context, userInput string) string {
	if p == nil || p.session == nil {
		return ""
	}
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return ""
	}
	ctxStr := "用户诉求: " + userInput + "\n"
	res, err := p.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_retrieve",
		Arguments: map[string]any{
			"context":    ctxStr,
			"query_hint": userInput,
		},
	})
	if err != nil {
		fmt.Printf("[plan/memoryhook] OnTurnRetrieve 失败（已忽略）: %v\n", err)
		return ""
	}
	return extractHintsField(extractMCPText(res))
}
