// 从 AgentTest 配置启动并冒烟 python-sandbox MCP（不跑 Plan/Behavior 全链路）。
// 用法（仓库根目录）：go run ./scripts/python_sandbox_smoke
package main

import (
	"AgentTest/capabilities"
	"AgentTest/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	cfgPath := os.Getenv("AGENTTEST_CONFIG")
	if cfgPath == "" {
		cfgPath = "config/app.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	config.SetGlobal(cfg)

	exe := filepath.Join(cfg.AbsRoot(), "WorkSpace", "mcp_bundled", "mcp-python-exec", "mcp-python-exec.exe")
	if _, err := os.Stat(exe); err != nil {
		fmt.Fprintf(os.Stderr, "missing bundled exe: %s\n", exe)
		os.Exit(1)
	}
	ws := cfg.ResolvedPaths().Workspace
	fmt.Printf("workspace=%s\nexe=%s\n", ws, exe)

	for _, s := range cfg.Capabilities.MCP.Servers {
		if s.Name == "python-sandbox" && s.Enabled {
			fmt.Printf("config: workdir=%q PYEXEC_WORKDIR=%s RESTRICT=%s\n",
				s.WorkDir, s.Env["PYEXEC_WORKDIR"], s.Env["PYEXEC_RESTRICT_PATH"])
		}
	}

	ctx := context.Background()
	if err := capabilities.Start(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "capabilities start: %v\n", err)
		os.Exit(1)
	}
	defer capabilities.Close()
	fmt.Println("OK: capabilities.Start completed (see log lines for python-sandbox tool count)")
}
