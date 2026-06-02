// 一次性检查：config/app.yaml + memory-mcp.exe + InitAgents（含 MCP stdio 连接）。
package main

import (
	"AgentTest/agent"
	"AgentTest/config"
	"fmt"
	"os"
)

func main() {
	cfgPath := os.Getenv("AGENTTEST_CONFIG")
	if cfgPath == "" {
		cfgPath = "config/app.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(1)
	}
	config.SetGlobal(cfg)
	if err := agent.InitAgents(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK: InitAgents (plan_memory_hook mcp connected)")
}
