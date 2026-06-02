package prefrontalCortex

import (
	"encoding/json"
	"regexp"
	"strings"
)

// PlanStep 单步执行：mcp=直接 Action 名；skill=SetExecutorStep；details=get_capability_details。
type PlanStep struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Args string `json:"args,omitempty"`
}

// ToolPlan 结构化工具计划（可由规则引擎或模型 JSON 产出）。
type ToolPlan struct {
	Intent string     `json:"intent,omitempty"`
	Steps  []PlanStep `json:"steps"`
}

var reToolPlanJSON = regexp.MustCompile(`(?is)\{[\s\n]*"(?:intent|steps)"[\s\S]*\}`)

// ParseToolPlanJSON 从模型输出中提取 ToolPlan JSON。
func ParseToolPlanJSON(answer string) (*ToolPlan, bool) {
	candidates := []string{strings.TrimSpace(answer)}
	if m := reToolPlanJSON.FindString(answer); m != "" {
		candidates = append([]string{m}, candidates...)
	}
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "{") {
			continue
		}
		var plan ToolPlan
		if err := json.Unmarshal([]byte(raw), &plan); err != nil {
			continue
		}
		if len(plan.Steps) == 0 {
			continue
		}
		for i := range plan.Steps {
			plan.Steps[i].Kind = strings.ToLower(strings.TrimSpace(plan.Steps[i].Kind))
			plan.Steps[i].Name = strings.TrimSpace(plan.Steps[i].Name)
			if plan.Steps[i].Args == "" {
				plan.Steps[i].Args = "{}"
			}
		}
		return &plan, true
	}
	return nil, false
}

// PlanFromActions 将 ReAct Action 块转为统一步骤（便于与 ToolPlan 共用执行器）。
func PlanFromActions(actions []struct{ Name, Params string }) ToolPlan {
	steps := make([]PlanStep, 0, len(actions))
	for _, a := range actions {
		name := strings.TrimSpace(a.Name)
		params := strings.TrimSpace(a.Params)
		if params == "" {
			params = "{}"
		}
		switch name {
		case "SetExecutorStep":
			steps = append(steps, PlanStep{Kind: "skill", Name: name, Args: params})
		case "get_capability_details":
			steps = append(steps, PlanStep{Kind: "details", Name: name, Args: params})
		default:
			if strings.Contains(name, "__") {
				steps = append(steps, PlanStep{Kind: "mcp", Name: name, Args: params})
			} else {
				steps = append(steps, PlanStep{Kind: "tool", Name: name, Args: params})
			}
		}
	}
	return ToolPlan{Steps: steps}
}

// ResolvePlanSteps 将 PlanStep 解析为可执行的 tool 名 + JSON 参数。
func ResolvePlanSteps(steps []PlanStep) []struct{ Name, Params string } {
	out := make([]struct{ Name, Params string }, 0, len(steps))
	for _, s := range steps {
		kind := strings.ToLower(strings.TrimSpace(s.Kind))
		name := strings.TrimSpace(s.Name)
		args := strings.TrimSpace(s.Args)
		if args == "" {
			args = "{}"
		}
		switch kind {
		case "mcp", "tool":
			out = append(out, struct{ Name, Params string }{Name: name, Params: args})
		case "skill":
			if name == "SetExecutorStep" {
				out = append(out, struct{ Name, Params string }{Name: name, Params: args})
			} else {
				payload, _ := json.Marshal(map[string]any{"skill": name, "args": parseArgsArray(args)})
				out = append(out, struct{ Name, Params string }{Name: "SetExecutorStep", Params: string(payload)})
			}
		case "details":
			out = append(out, struct{ Name, Params string }{Name: "get_capability_details", Params: args})
		default:
			if name != "" {
				out = append(out, struct{ Name, Params string }{Name: name, Params: args})
			}
		}
	}
	return out
}

func parseArgsArray(argsJSON string) []any {
	var wrap struct {
		Args []any `json:"args"`
	}
	if json.Unmarshal([]byte(argsJSON), &wrap) == nil && len(wrap.Args) > 0 {
		return wrap.Args
	}
	var arr []any
	if json.Unmarshal([]byte(argsJSON), &arr) == nil {
		return arr
	}
	return nil
}
