// 快速路由探测：先 seed MCP 事实库，再测 DecideRoute（见 memory_boundary_test）。
// 用法：go run ./scripts/check_memory_route
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	args := []string{"run", "./scripts/memory_boundary_test", "-config", configPath()}
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

func configPath() string {
	if p := os.Getenv("AGENTTEST_CONFIG"); p != "" {
		return p
	}
	return "config/app.yaml"
}
