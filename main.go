package main

import "C"
import (
	"AgentTest/agent"
	"AgentTest/agentWorkSpace/workSpace"
	"AgentTest/capabilities"
	"AgentTest/config"
	"AgentTest/skillpacks"
	"context"
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
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[config] 已加载 %s (root=%s)\n", cfgPath, cfg.Root)
	if err := skillpacks.Apply(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "skill_packs: %v\n", err)
	}
	config.SetGlobal(cfg)

	if err := capabilities.Start(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "capabilities 启动: %v\n", err)
	}
	defer capabilities.Close()

	if err := agent.InitAgents(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "初始化 Agent 失败: %v\n", err)
		os.Exit(1)
	}

	workSpace.WorkStart()
	//community.WorkStartCommunity()
}
