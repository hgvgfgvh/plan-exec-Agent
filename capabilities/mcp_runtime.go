package capabilities

import (
	"AgentTest/config"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/tools"
	"golang.org/x/time/rate"
)

var (
	mcpMu            sync.Mutex
	mcpTools         []tools.Tool
	mcpCatalog       []mcpToolCatalogEntry
	mcpServerCatalog []mcpServerCatalogEntry
	mcpClosers       []func()
	mcpStarted       bool
	mcpLimiters      sync.Map // server name -> *rate.Limiter
)

// mcpServerCatalogEntry 第一层 MCP 服务摘要（不含逐工具 Schema）。
type mcpServerCatalogEntry struct {
	ServerName string
	Brief      string
	ToolCount  int
}

// mcpToolCatalogEntry MCP 工具目录项：第一层摘要 + 第二层 FullDoc。
type mcpToolCatalogEntry struct {
	PublicName   string
	ServerName   string
	OrigName     string
	ShortSummary string
	FullDoc      string
	InputSchema  map[string]any
}

// Start 连接配置中启用的 MCP Server（stdio 或 streamable HTTP），注册工具。
func Start(ctx context.Context, cfg *config.App) error {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	if mcpStarted {
		return nil
	}
	mcpStarted = true
	if cfg == nil || !cfg.Capabilities.MCP.Enabled {
		return nil
	}

	mcpTools = nil
	mcpCatalog = nil
	mcpServerCatalog = nil
	usedNames := make(map[string]bool)
	for _, sdef := range cfg.Capabilities.MCP.Servers {
		if !sdef.Enabled {
			continue
		}
		if sdef.Name == "" {
			fmt.Printf("[capabilities] MCP server 缺少 name，跳过\n")
			continue
		}
		if !mcpServerAllowed(cfg, sdef.Name) {
			fmt.Printf("[capabilities] MCP server %q 不在 allow_mcp_server_names 中，跳过\n", sdef.Name)
			continue
		}

		sess, err := connectMCPClient(ctx, cfg, sdef)
		if err != nil {
			fmt.Printf("[capabilities] MCP 连接失败 server=%s: %v\n", sdef.Name, err)
			continue
		}

		toolDefs, err := listAllTools(ctx, sess)
		if err != nil {
			_ = sess.Close()
			fmt.Printf("[capabilities] MCP tools/list 失败 server=%s: %v\n", sdef.Name, err)
			continue
		}

		var lim *rate.Limiter
		if sdef.RatePerMinute > 0 {
			v, _ := mcpLimiters.LoadOrStore(sdef.Name, rate.NewLimiter(rate.Limit(float64(sdef.RatePerMinute)/60.0), 1))
			lim = v.(*rate.Limiter)
		}
		maxOut := cfg.Capabilities.Security.MCPMaxOutputChars
		registered := 0
		for _, td := range toolDefs {
			if td == nil || td.Name == "" {
				continue
			}
			pub := uniquePublicToolName(sdef.Name, td.Name, usedNames)
			if mcpToolDenied(cfg, pub) {
				fmt.Printf("[capabilities] 跳过 MCP 工具（策略拒绝）: %s\n", pub)
				continue
			}
			full := buildToolFullDocumentation(pub, td)
			mcpCatalog = append(mcpCatalog, mcpToolCatalogEntry{
				PublicName:   pub,
				ServerName:   sdef.Name,
				OrigName:     td.Name,
				ShortSummary: firstLineSummary(td.Description, 200),
				FullDoc:      full,
				InputSchema:  ParseInputSchemaJSON(td.InputSchema),
			})
			mcpTools = append(mcpTools, &mcpLangchainTool{
				session:    sess,
				serverName: sdef.Name,
				publicName: pub,
				origName:   td.Name,
				limiter:    lim,
				maxOutput:  maxOut,
			})
			registered++
		}
		mcpClosers = append(mcpClosers, func() {
			_ = sess.Close()
		})
		brief := strings.TrimSpace(sdef.Description)
		if brief == "" {
			brief = inferMCPServerBrief(sdef.Name, toolDefs)
		}
		mcpServerCatalog = append(mcpServerCatalog, mcpServerCatalogEntry{
			ServerName: sdef.Name,
			Brief:      brief,
			ToolCount:  registered,
		})
		fmt.Printf("[capabilities] MCP 已连接 server=%s transport=%s tools_registered=%d/%d\n",
			sdef.Name, inferTransportLabel(sdef), registered, len(toolDefs))
	}
	return nil
}

func inferMCPServerBrief(serverName string, toolDefs []*mcpsdk.Tool) string {
	switch strings.ToLower(strings.TrimSpace(serverName)) {
	case "filesystem":
		return "本地文件与目录：读写、搜索、元数据、目录树等"
	case "resend":
		return "邮件与营销：发信、联系人、模板、广播、域名、Webhook 等"
	case "sqlite":
		return "SQLite 数据库：表结构、SQL 查询与写入"
	}
	if len(toolDefs) > 0 {
		if line := firstLineSummary(toolDefs[0].Description, 100); line != "" {
			return fmt.Sprintf("%s（共 %d 个工具）", line, len(toolDefs))
		}
	}
	return fmt.Sprintf("MCP 服务（%d 个工具）", len(toolDefs))
}

func inferTransportLabel(sdef config.MCPServerDef) string {
	if strings.TrimSpace(sdef.Endpoint) != "" {
		return "http"
	}
	return "stdio"
}

// Close 关闭所有 MCP 会话（可重复调用）。
func Close() {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	if !mcpStarted {
		return
	}
	for i := len(mcpClosers) - 1; i >= 0; i-- {
		mcpClosers[i]()
	}
	mcpClosers = nil
	mcpTools = nil
	mcpCatalog = nil
	mcpServerCatalog = nil
	mcpStarted = false
	mcpLimiters.Range(func(key, _ any) bool {
		mcpLimiters.Delete(key)
		return true
	})
}

// AppendToolsForAgent 若 agentKey 在 capabilities.attach_to 中，则在 base 后追加 MCP 工具与本机 RegisterLangchainTools 工具。
func AppendToolsForAgent(agentKey string, base []tools.Tool) []tools.Tool {
	cfg := config.Get()
	if !shouldAttach(cfg, agentKey) {
		return base
	}
	mcpMu.Lock()
	extras := cloneExtraLangchainTools()
	extras = append([]tools.Tool{getCapabilityDetailsTool{}}, extras...)
	if cfg.Capabilities.MCP.Enabled {
		extras = append(extras, mcpTools...)
	}
	mcpMu.Unlock()
	if len(extras) == 0 {
		return base
	}
	out := append([]tools.Tool{}, base...)
	return append(out, extras...)
}

func shouldAttach(cfg *config.App, agentKey string) bool {
	for _, k := range cfg.Capabilities.AttachTo {
		if k == agentKey {
			return true
		}
	}
	return false
}

func listAllTools(ctx context.Context, s *mcpsdk.ClientSession) ([]*mcpsdk.Tool, error) {
	var out []*mcpsdk.Tool
	cursor := ""
	for {
		res, err := s.ListTools(ctx, &mcpsdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		out = append(out, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return out, nil
}

func mangleIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "tool"
	}
	return b.String()
}

func uniquePublicToolName(serverLabel, mcpToolName string, used map[string]bool) string {
	prefix := mangleIdent(serverLabel)
	base := mangleIdent(mcpToolName)
	candidate := prefix + "__" + base
	if !used[candidate] {
		used[candidate] = true
		return candidate
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s__%s_%d", prefix, base, i)
		if !used[c] {
			used[c] = true
			return c
		}
	}
}

func buildToolFullDocumentation(publicName string, t *mcpsdk.Tool) string {
	var b strings.Builder
	b.WriteString("公开名: ")
	b.WriteString(publicName)
	b.WriteString("\n远端工具名: ")
	b.WriteString(t.Name)
	b.WriteByte('\n')
	if strings.TrimSpace(t.Description) != "" {
		b.WriteString(t.Description)
		b.WriteByte('\n')
	}
	if t.InputSchema != nil {
		raw, err := json.Marshal(t.InputSchema)
		if err == nil && len(raw) > 0 {
			b.WriteString("参数 JSON Schema:\n")
			if len(raw) > 12000 {
				b.WriteString(string(raw[:12000]))
				b.WriteString("\n…(Schema 已截断，必要时在 MCP Server 侧查看完整定义)")
			} else {
				b.Write(raw)
			}
		}
	}
	return b.String()
}

type mcpLangchainTool struct {
	session    *mcpsdk.ClientSession
	serverName string
	publicName string
	origName   string
	limiter    *rate.Limiter
	maxOutput  int
}

func (t *mcpLangchainTool) Name() string { return t.publicName }

// SuppressExecutorToolPrompt MCP 工具不在每轮 system 的工具清单中枚举；见 system 内 Agent 能力目录。
func (*mcpLangchainTool) SuppressExecutorToolPrompt() bool { return true }

// Description 供 LangChain 元数据与日志使用；执行器清单已抑制 MCP 行，此处仍保持极简。
func (t *mcpLangchainTool) Description() string {
	return fmt.Sprintf("MCP（server=%s，远端=%s）；公开名与摘要见 system 能力目录；详情用 get_capability_details。", t.serverName, t.origName)
}

func (t *mcpLangchainTool) Call(ctx context.Context, input string) (string, error) {
	cfg := config.Get()
	start := time.Now()
	if t.limiter != nil {
		if err := t.limiter.Wait(ctx); err != nil {
			return "", err
		}
	}
	args := map[string]any{}
	in := strings.TrimSpace(input)
	if in != "" {
		if err := json.Unmarshal([]byte(in), &args); err != nil {
			return "", fmt.Errorf("MCP 工具 %q 需要 JSON 对象参数: %w", t.publicName, err)
		}
	}
	CoerceMCPArguments(t.publicName, args)
	res, err := t.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      t.origName,
		Arguments: args,
	})
	d := time.Since(start)
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	out := ""
	if err == nil {
		out = formatCallToolResult(res)
		out = trimMCPOutput(out, t.maxOutput)
	}
	WriteAudit(cfg, map[string]any{
		"kind":        "mcp_tool",
		"server":      t.serverName,
		"tool":        t.origName,
		"public_name": t.publicName,
		"duration_ms": d.Milliseconds(),
		"error":       errStr,
		"output_len":  len(out),
	})
	if err != nil {
		return "", err
	}
	return out, nil
}

func trimMCPOutput(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n…(输出已按 mcp_max_output_chars 截断)"
}

func formatCallToolResult(res *mcpsdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		switch v := c.(type) {
		case *mcpsdk.TextContent:
			b.WriteString(v.Text)
		default:
			raw, _ := json.Marshal(v)
			b.WriteString(string(raw))
		}
	}
	if res.StructuredContent != nil {
		raw, _ := json.Marshal(res.StructuredContent)
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.Write(raw)
	}
	s := strings.TrimSpace(b.String())
	if res.IsError {
		if s == "" {
			return "MCP 工具返回错误（无正文）"
		}
		return "MCP 工具错误: " + s
	}
	if s == "" {
		return "(MCP 无文本输出)"
	}
	return s
}

var _ tools.Tool = (*mcpLangchainTool)(nil)
