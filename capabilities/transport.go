package capabilities

import (
	"AgentTest/config"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type headerRoundTripper struct {
	base http.RoundTripper
	h    http.Header
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, vals := range h.h {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	return h.base.RoundTrip(req)
}

func httpClientForMCP(headers map[string]string) *http.Client {
	base := http.DefaultTransport
	var rt http.RoundTripper = base
	if len(headers) > 0 {
		h := http.Header{}
		for k, v := range headers {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			h.Add(k, v)
		}
		if len(h) > 0 {
			rt = &headerRoundTripper{base: base, h: h}
		}
	}
	return &http.Client{
		Timeout:   4 * time.Minute,
		Transport: rt,
	}
}

// buildMCPTransport 构造 stdio 或 streamable HTTP 传输层。
func buildMCPTransport(ctx context.Context, cfg *config.App, sdef config.MCPServerDef) (mcpsdk.Transport, error) {
	mode := strings.ToLower(strings.TrimSpace(sdef.Transport))
	if mode == "" && strings.TrimSpace(sdef.Endpoint) != "" {
		mode = "http"
	}
	switch mode {
	case "http", "https", "streamable", "sse":
		ep := strings.TrimSpace(sdef.Endpoint)
		if ep == "" {
			return nil, fmt.Errorf("server %q: endpoint 为空", sdef.Name)
		}
		maxR := sdef.HTTPMaxRetries
		if maxR == 0 {
			maxR = 5
		}
		return &mcpsdk.StreamableClientTransport{
			Endpoint:             ep,
			HTTPClient:           httpClientForMCP(sdef.Headers),
			MaxRetries:           maxR,
			DisableStandaloneSSE: sdef.DisableStandaloneSSE,
		}, nil
	default:
		if strings.TrimSpace(sdef.Command) == "" {
			return nil, fmt.Errorf("server %q: stdio 需要 command", sdef.Name)
		}
		cmdPath := resolveMCPStdioCommand(cfg, sdef.Command)
		args := resolveMCPStdioArgs(cfg, sdef.Args)
		cmd := exec.CommandContext(ctx, cmdPath, args...)
		if wd := strings.TrimSpace(sdef.WorkDir); wd != "" {
			wd = filepath.Clean(filepath.FromSlash(wd))
			if filepath.IsAbs(wd) {
				cmd.Dir = wd
			} else {
				cmd.Dir = filepath.Join(cfg.AbsRoot(), wd)
			}
		}
		cmd.Env = os.Environ()
		for k, v := range sdef.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		return &mcpsdk.CommandTransport{Command: cmd}, nil
	}
}

// resolveMCPStdioCommand：相对应用 root 的 MCP 可执行路径（如 WorkSpace/mcp_bundled/.../server.exe）解析为绝对路径，便于打包分发；单文件名仍走 PATH。
func resolveMCPStdioCommand(cfg *config.App, command string) string {
	command = strings.TrimSpace(command)
	if cfg == nil || command == "" || filepath.IsAbs(command) {
		return command
	}
	if !strings.ContainsAny(command, `/\`) {
		return command
	}
	return filepath.Join(cfg.AbsRoot(), filepath.Clean(filepath.FromSlash(command)))
}

// resolveMCPStdioArgs：将「仓库内真实存在的」相对路径参数转为绝对路径；跳过以 '-' 开头的开关、以及磁盘上不存在的拼接结果（避免误改 npx 包名等）。
func resolveMCPStdioArgs(cfg *config.App, args []string) []string {
	if cfg == nil || len(args) == 0 {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = resolveMCPStdioArg(cfg, a)
	}
	return out
}

func resolveMCPStdioArg(cfg *config.App, s string) string {
	s = strings.TrimSpace(s)
	if s == "" || filepath.IsAbs(s) {
		return s
	}
	if strings.HasPrefix(s, "-") {
		return s
	}
	if !strings.ContainsAny(s, `/\`) {
		cand := filepath.Join(cfg.AbsRoot(), filepath.Clean(filepath.FromSlash(s)))
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		return s
	}
	cand := filepath.Join(cfg.AbsRoot(), filepath.Clean(filepath.FromSlash(s)))
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return s
}

func connectMCPClient(ctx context.Context, cfg *config.App, sdef config.MCPServerDef) (*mcpsdk.ClientSession, error) {
	transport, err := buildMCPTransport(ctx, cfg, sdef)
	if err != nil {
		return nil, err
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "AgentTest", Version: "0.1"}, nil)
	return client.Connect(ctx, transport, nil)
}
