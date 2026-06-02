package prefrontalCortex

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/capabilities"
	"AgentTest/config"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmc/langchaingo/tools"
)

func normalizeWorkspacePrefixedPath(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return s
	}
	// MCP filesystem/documents 运行时常以 WorkSpace 作为根目录；参数里再带 WorkSpace/ 会导致双前缀。
	const (
		p1 = "WorkSpace/"
		p2 = "WorkSpace\\"
	)
	if strings.HasPrefix(t, p1) {
		return strings.TrimPrefix(t, p1)
	}
	if strings.HasPrefix(t, p2) {
		return strings.TrimPrefix(t, p2)
	}
	return s
}

func normalizeMCPParams(toolName, params string) string {
	// 仅处理常见 path/outputPath 等字段，避免模型在 tool 参数里写 WorkSpace/ 前缀导致 MCP 根目录再拼一层。
	if !strings.HasPrefix(toolName, "filesystem__") && !strings.HasPrefix(toolName, "documents__") && toolName != "resend__send_email" {
		return params
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(params), &m); err != nil {
		return params
	}
	changed := false
	normalizeField := func(k string) {
		raw, ok := m[k]
		if !ok || raw == nil {
			return
		}
		if s, ok := raw.(string); ok {
			ns := normalizeWorkspacePrefixedPath(s)
			if ns != s {
				m[k] = ns
				changed = true
			}
		}
	}
	normalizeStringSlice := func(k string) {
		raw, ok := m[k]
		if !ok || raw == nil {
			return
		}
		arr, ok := raw.([]any)
		if !ok {
			return
		}
		for i := range arr {
			if s, ok := arr[i].(string); ok {
				ns := normalizeWorkspacePrefixedPath(s)
				if ns != s {
					arr[i] = ns
					changed = true
				}
			}
		}
		m[k] = arr
	}

	// filesystem
	normalizeField("path")
	normalizeStringSlice("paths")
	// documents
	normalizeField("outputPath")
	normalizeField("filePath")
	// resend attachments can reference local filePath
	if toolName == "resend__send_email" {
		// attachments: [{ filename, filePath, ... }]
		if raw, ok := m["attachments"]; ok {
			if arr, ok := raw.([]any); ok {
				for i := range arr {
					obj, ok := arr[i].(map[string]any)
					if !ok {
						continue
					}
					if fp, ok := obj["filePath"].(string); ok {
						nfp := normalizeWorkspacePrefixedPath(fp)
						if nfp != fp {
							obj["filePath"] = nfp
							changed = true
						}
					}
					arr[i] = obj
				}
				m["attachments"] = arr
			}
		}
	}

	if !changed {
		return params
	}
	b, err := json.Marshal(m)
	if err != nil {
		return params
	}
	return string(b)
}

// executeToolBatch 执行一批已解析的 Action（含熔断、并行策略）。
func (e *CustomExecutor) executeToolBatch(
	ctx context.Context,
	actions []struct{ Name, Params string },
	executedActions map[string]bool,
	archiveCallCount *int,
	lineMaxRunes int,
	revealed *capabilities.RevealedToolSet,
	progressive bool,
) (obsStr string, executedToolNames []string, err error) {
	type preparedAct struct {
		Idx      int
		Name     string
		Params   string
		Skip     bool
		SkipLine string
		Tool     tools.Tool
	}

	prepared := make([]preparedAct, 0, len(actions))
	batchHasUnsafe := false
	for idx, act := range actions {
		act.Params = normalizeMCPParams(act.Name, act.Params)
		p := preparedAct{Idx: idx + 1, Name: act.Name, Params: act.Params}
		if act.Name == "archive_specific_rounds" {
			*archiveCallCount++
			if *archiveCallCount > 3 {
				p.Skip = true
				p.SkipLine = fmt.Sprintf("%d. 工具 [%s] 结果: 警告！清理操作过于频繁。单次任务清理上限为3次。\n", p.Idx, act.Name)
				prepared = append(prepared, p)
				continue
			}
		} else if act.Name != "" {
			*archiveCallCount = 0
		}
		tool, exists := e.Tools[act.Name]
		executedToolNames = append(executedToolNames, act.Name)
		if !exists {
			p.Skip = true
			p.SkipLine = fmt.Sprintf("%d. 工具 [%s] 结果: 错误：工具不存在\n", p.Idx, act.Name)
			prepared = append(prepared, p)
			continue
		}
		if capabilities.MCPRequiresReveal(act.Name, tool, progressive, revealed) {
			p.Skip = true
			p.SkipLine = fmt.Sprintf("%d. 工具 [%s] 结果: 须先调用 get_capability_details 解锁该 MCP（mcp_tools 填公开名或 server 名），再执行。\n", p.Idx, act.Name)
			prepared = append(prepared, p)
			continue
		}

		// 只有当工具真实可执行时才计入“重复调用”去重，
		// 避免「未解锁」导致的失败调用占用签名，使解锁后重试被误判为重复。
		actionKey := act.Name + act.Params
		if executedActions[actionKey] {
			p.Skip = true
			p.SkipLine = fmt.Sprintf("%d. 工具 [%s]: 跳过，严禁重复调用相同参数的工具。\n", p.Idx, act.Name)
			prepared = append(prepared, p)
			continue
		}
		executedActions[actionKey] = true

		p.Tool = tool
		if _, unsafe := tool.(capabilities.ExecutorParallelUnsafe); unsafe {
			batchHasUnsafe = true
		}
		prepared = append(prepared, p)
	}

	lines := make([]string, len(prepared))
	execute := func(i int, p preparedAct) {
		fmt.Printf("---------------->>> 状态: 执行工具 %d/%d [%s]\n", p.Idx, len(prepared), p.Name)
		t0 := time.Now()
		obs, callErr := p.Tool.Call(ctx, p.Params)
		dur := time.Since(t0)
		if cfg := config.TryGet(); cfg != nil && cfg.Capabilities.Observability.Enabled && cfg.Capabilities.Observability.NativeToolCalls {
			capabilities.LogNativeTool(e.AgentName, p.Name, dur, callErr, len(obs), p.Params)
		}
		if callErr != nil {
			obs = "执行失败: " + callErr.Error()
		} else if p.Name == "get_capability_details" && revealed != nil {
			if added := revealed.RevealFromDetailsJSON(p.Params); len(added) > 0 {
				fmt.Printf("[executor] 渐进披露：已解锁 MCP tools (%d): %s\n", len(added), strings.Join(added, ", "))
			}
		} else if p.Name == "BehaviorAgentAgentOutput" {
			atomic.StoreUint32(&e.behaviorOutputPublished, 1)
		}
		if runcontrol.IsPlanControlledExecution(ctx) {
			if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
				runcontrol.RecordPlanToolCall(turnID, p.Name)
			}
		}
		fmt.Printf("%d. 工具 [%s] 结果: %s\n", p.Idx, p.Name, obs)
		lines[i] = fmt.Sprintf("%d. 工具 [%s] 结果: %s\n", p.Idx, p.Name, truncateRunes(obs, lineMaxRunes))
	}

	if batchHasUnsafe {
		for i := range prepared {
			if prepared[i].Skip {
				lines[i] = prepared[i].SkipLine
				continue
			}
			execute(i, prepared[i])
		}
	} else {
		var wg sync.WaitGroup
		for i := range prepared {
			if prepared[i].Skip {
				lines[i] = prepared[i].SkipLine
				continue
			}
			wg.Add(1)
			go func(i int, p preparedAct) {
				defer wg.Done()
				execute(i, p)
			}(i, prepared[i])
		}
		wg.Wait()
	}

	var observations strings.Builder
	observations.WriteString("【多个工具并行返回结果】:\n")
	for _, ln := range lines {
		observations.WriteString(ln)
	}
	return observations.String(), executedToolNames, nil
}
