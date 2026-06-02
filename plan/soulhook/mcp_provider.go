package soulhook

import (
	"AgentTest/config"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	RegisterProvider("mcp", newMCPProvider)
}

// MCPProvider 通过 stdio 连接 AgentTestSoulMCP（soul_store / soul_retrieve）。
type MCPProvider struct {
	session *mcpsdk.ClientSession
}

func newMCPProvider(cfg *config.App) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("soulhook mcp: nil config")
	}
	if !cfg.PlanSoulHook.Enabled {
		return NoopProvider{}, nil
	}
	cmd := strings.TrimSpace(cfg.PlanSoulHook.MCPCommand)
	if cmd == "" {
		return nil, fmt.Errorf("soulhook mcp: plan_soul_hook.mcp_command 未配置")
	}
	cmd = resolveCommandPath(cfg, cmd)
	ctx := context.Background()
	execCmd := exec.CommandContext(ctx, cmd)
	execCmd.Env = os.Environ()
	for k, v := range cfg.PlanSoulHook.MCPEnv {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}
	transport := &mcpsdk.CommandTransport{Command: execCmd}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "AgentTest-soulhook", Version: "0.1"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("soulhook mcp connect: %w", err)
	}
	return &MCPProvider{session: sess}, nil
}

func (p *MCPProvider) Name() string { return "mcp" }

func (p *MCPProvider) RetrieveHints(ctx context.Context, userInput string) string {
	if p == nil || p.session == nil {
		return ""
	}
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return ""
	}
	ctxStr := "用户输入: " + userInput + "\n"
	res, err := p.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "soul_retrieve",
		Arguments: map[string]any{
			"context":    ctxStr,
			"query_hint": userInput,
		},
	})
	if err != nil {
		fmt.Printf("[plan/soulhook] soul_retrieve 失败（已忽略）: %v\n", err)
		return ""
	}
	return extractHintsField(extractMCPText(res))
}

func (p *MCPProvider) StoreDialogue(ctx context.Context, in DialogueStoreInput, content string) error {
	if p == nil || p.session == nil {
		return fmt.Errorf("soulhook mcp: no session")
	}
	correlation := strings.TrimSpace(in.TurnID)
	if correlation == "" {
		correlation = "webui-turn"
	}
	_, err := p.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "soul_store",
		Arguments: map[string]any{
			"content":        content,
			"source":         "agenttest-webui",
			"kind":           "dialogue",
			"correlation_id": correlation,
		},
	})
	return err
}

var _ DialogueStorer = (*MCPProvider)(nil)

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

// extractHintsField 从 soul_retrieve JSON 取出 hints；非 JSON 则原样返回。
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
