package capabilities

import (
	"AgentTest/config"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tmc/langchaingo/tools"
)

// alwaysExposedToolNames 渐进披露下始终出现在 API tools[] 中的工具（地图/元能力/内置执行入口）。
var alwaysExposedToolNames = map[string]bool{
	"get_capability_details":  true,
	"list_agent_capabilities": true,
	"SetExecutorStep":         true,
	"report_step_result":      true,
	"AbortExecutorStep":       true,
}

// RevealedToolSet 单次 CustomExecutor.Run 内动态解锁的 MCP 公开名；Run 结束即丢弃。
type RevealedToolSet struct {
	mu    sync.Mutex
	names map[string]bool
}

func NewRevealedToolSet() *RevealedToolSet {
	return &RevealedToolSet{names: make(map[string]bool)}
}

func (r *RevealedToolSet) Has(name string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.names[name]
}

func (r *RevealedToolSet) add(name string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	r.names[name] = true
	r.mu.Unlock()
}

// Names 返回已解锁 MCP 名（排序后），供调试日志。
func (r *RevealedToolSet) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.names))
	for n := range r.names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// RevealFromDetailsJSON 解析 get_capability_details 入参并解锁对应 MCP tool_calls。
func (r *RevealedToolSet) RevealFromDetailsJSON(input string) []string {
	req, err := parseCapabilityDetailsInput(input)
	if err != nil {
		return nil
	}
	return r.RevealMCPTools(req.MCPTools)
}

// RevealMCPTools mcp_tools 可为公开名或 MCP server 名（展开该服务下全部工具）。
func (r *RevealedToolSet) RevealMCPTools(names []string) []string {
	if r == nil {
		return nil
	}
	var added []string
	seen := make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if strings.Contains(name, "__") {
			if _, ok := mcpCatalogFullDoc(name); ok {
				if !seen[name] {
					r.add(name)
					added = append(added, name)
					seen[name] = true
				}
			}
			continue
		}
		for _, e := range snapshotMCPCatalog() {
			if e.ServerName != name {
				continue
			}
			if seen[e.PublicName] {
				continue
			}
			r.add(e.PublicName)
			added = append(added, e.PublicName)
			seen[e.PublicName] = true
		}
	}
	sort.Strings(added)
	return added
}

func parseCapabilityDetailsInput(input string) (capabilityDetailsInput, error) {
	var req capabilityDetailsInput
	in := strings.TrimSpace(input)
	if in == "" {
		return req, fmt.Errorf("empty input")
	}
	if err := json.Unmarshal([]byte(in), &req); err != nil {
		return req, err
	}
	return req, nil
}

// IsMCPTool 是否为 MCP 注册工具（SuppressExecutorToolPrompt）。
func IsMCPTool(t tools.Tool) bool {
	if t == nil {
		return false
	}
	if sup, ok := t.(ExecutorToolPromptSuppressor); ok && sup.SuppressExecutorToolPrompt() {
		return true
	}
	return false
}

func isAlwaysExposedAPI(name string, t tools.Tool) bool {
	if alwaysExposedToolNames[name] {
		return true
	}
	if IsMCPTool(t) {
		return false
	}
	return true
}

// FilterToolMapForAPI 渐进披露：仅保留常显工具 + 已解锁 MCP；full 仍用于实际 Call。
func FilterToolMapForAPI(full map[string]tools.Tool, revealed *RevealedToolSet, progressive bool) map[string]tools.Tool {
	if !progressive || len(full) == 0 {
		return full
	}
	out := make(map[string]tools.Tool, len(full))
	for name, t := range full {
		if isAlwaysExposedAPI(name, t) {
			out[name] = t
			continue
		}
		if IsMCPTool(t) && revealed != nil && revealed.Has(name) {
			out[name] = t
		}
	}
	return out
}

// UseProgressiveToolDisclosure 是否对指定 Executor 启用 API tools[] 渐进披露。
func UseProgressiveToolDisclosure(executorAgentName string) bool {
	if !CatalogAttachedToExecutor(executorAgentName) {
		return false
	}
	cfg := config.TryGet()
	if cfg == nil {
		return false
	}
	if cfg.Executor.DisableProgressiveToolDisclosure {
		return false
	}
	return cfg.Capabilities.MCP.Enabled
}

// MCPRequiresReveal 渐进披露下是否须先 get_capability_details 解锁。
func MCPRequiresReveal(toolName string, t tools.Tool, progressive bool, revealed *RevealedToolSet) bool {
	if !progressive || !IsMCPTool(t) {
		return false
	}
	return revealed == nil || !revealed.Has(toolName)
}
