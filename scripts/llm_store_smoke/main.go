// 验证 Phase-2c LLM 抽取：读取 app.yaml 的 mcp_env，store 一条 episode，检查 atoms.jsonl。
// 用法：go run ./scripts/llm_store_smoke
// 可选：-mcp 指定 memory-mcp 可执行文件（默认 config 中的路径，若存在 memory-mcp-new.exe 则优先）
package main

import (
	"AgentTest/config"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	mcpExe := flag.String("mcp", "", "memory-mcp 可执行文件路径")
	flag.Parse()

	cfgPath := os.Getenv("AGENTTEST_CONFIG")
	if cfgPath == "" {
		cfgPath = "config/app.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		exitErr("load config: %v", err)
	}
	if *mcpExe != "" {
		cfg.PlanMemoryHook.MCPCommand = *mcpExe
	} else if st, err := os.Stat(filepath.Join(filepath.Dir(cfg.PlanMemoryHook.MCPCommand), "memory-mcp-new.exe")); err == nil && !st.IsDir() {
		cfg.PlanMemoryHook.MCPCommand = filepath.Join(filepath.Dir(cfg.PlanMemoryHook.MCPCommand), "memory-mcp-new.exe")
		fmt.Println("using:", cfg.PlanMemoryHook.MCPCommand)
	}
	dataDir := cfg.PlanMemoryHook.MCPEnv["MEMORY_MCP_DATA_DIR"]
	atomsPath := filepath.Join(dataDir, "atoms", "atoms.jsonl")
	before := countLines(atomsPath)

	corr := fmt.Sprintf("llm-smoke-%d", time.Now().Unix())
	content := `[source=agenttest-plan turn=` + corr + ` plan=llm-smoke]

## 用户诉求
用 filesystem 列出 WorkSpace 根目录并写入 WorkSpace/llm_smoke_list.txt

## 门户回复
已完成 WorkSpace 目录列举，清单已写入 llm_smoke_list.txt。

## 计划终态 (TodoList)
status: completed
tools_called: filesystem__list_directory, filesystem__write_file
artifacts: WorkSpace/llm_smoke_list.txt
`
	sess, closeFn, stderr, err := connectMCP(cfg)
	if err != nil {
		exitErr("mcp connect: %v", err)
	}
	defer closeFn()

	ctx := context.Background()
	storeRaw, err := callTool(ctx, sess, "memory_store", map[string]any{
		"content": content, "source": "agenttest-plan", "kind": "episode",
		"correlation_id": corr,
	})
	if err != nil {
		exitErr("store: %v", err)
	}
	fmt.Println("store:", trim(storeRaw, 220))

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		after := countLines(atomsPath)
		if after > before {
			fmt.Printf("PASS: atoms.jsonl %d -> %d (LLM path OK)\n", before, after)
			showTail(atomsPath, 1)
			return
		}
	}
	after := countLines(atomsPath)
	fmt.Printf("FAIL or rules-only: atoms %d->%d facts_lines=%d\n", before, after, countLines(filepath.Join(dataDir, "facts", "facts.jsonl")))
	if stderr.Len() > 0 {
		fmt.Println("--- MCP stderr (tail) ---")
		fmt.Println(trim(stderr.String(), 2000))
	}
	os.Exit(1)
}

func connectMCP(cfg *config.App) (*mcpsdk.ClientSession, func(), *bytes.Buffer, error) {
	cmd := strings.TrimSpace(cfg.PlanMemoryHook.MCPCommand)
	engine := strings.TrimSpace(cfg.PlanMemoryHook.MCPEngine)
	if engine == "" {
		engine = "factworld"
	}
	execCmd := exec.Command(cmd, "-engine", engine)
	execCmd.Env = os.Environ()
	for k, v := range cfg.PlanMemoryHook.MCPEnv {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}
	var errBuf bytes.Buffer
	execCmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "llm-store-smoke", Version: "0.1"}, nil)
	sess, err := client.Connect(context.Background(), &mcpsdk.CommandTransport{Command: execCmd}, nil)
	if err != nil {
		return nil, nil, &errBuf, err
	}
	return sess, func() { _ = sess.Close() }, &errBuf, nil
}

func callTool(ctx context.Context, sess *mcpsdk.ClientSession, name string, args map[string]any) (string, error) {
	res, err := sess.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func countLines(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func showTail(path string, n int) {
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 {
		return
	}
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	for _, ln := range lines[start:] {
		var pretty map[string]any
		if json.Unmarshal([]byte(ln), &pretty) == nil {
			b2, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(b2))
		}
	}
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func exitErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
