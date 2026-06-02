package capabilities

import (
	"AgentTest/behavior/skill"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type getCapabilityDetailsTool struct{}

func (getCapabilityDetailsTool) Name() string { return "get_capability_details" }

func (getCapabilityDetailsTool) Description() string {
	return `获取 MCP Schema、外挂 SKILL 全文或内置技能参数说明。JSON：{"mcp_tools":[],"external_skills":[],"builtin_skills":[]}。
mcp_tools 可填 MCP server 名（如 filesystem，展开该服务全部工具）或公开工具名（如 filesystem__read_text_file）。可组合、可多项。`
}

type capabilityDetailsInput struct {
	MCPTools       []string `json:"mcp_tools"`
	ExternalSkills []string `json:"external_skills"`
	BuiltinSkills  []string `json:"builtin_skills"`
}

func (getCapabilityDetailsTool) Call(ctx context.Context, input string) (string, error) {
	_ = ctx
	in := strings.TrimSpace(input)
	if in == "" {
		return "", fmt.Errorf("请传 JSON：{\"mcp_tools\":[],\"external_skills\":[],\"builtin_skills\":[]}")
	}
	var req capabilityDetailsInput
	if err := json.Unmarshal([]byte(in), &req); err != nil {
		return "", fmt.Errorf("JSON 解析失败: %w", err)
	}
	if len(req.MCPTools) == 0 && len(req.ExternalSkills) == 0 && len(req.BuiltinSkills) == 0 {
		return "", fmt.Errorf("至少指定 mcp_tools、external_skills、builtin_skills 之一（勿传空数组 []）")
	}

	var b strings.Builder
	b.WriteString("# 能力详情（第二层）\n\n")

	if len(req.MCPTools) > 0 {
		for _, name := range req.MCPTools {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			doc, ok := resolveMCPDetailDoc(name)
			b.WriteString("## MCP: ")
			b.WriteString(name)
			b.WriteByte('\n')
			if !ok {
				b.WriteString("（未找到；mcp_tools 填 MCP server 名如 filesystem，或公开工具名如 filesystem__read_text_file）\n\n")
				continue
			}
			b.WriteString(doc)
			b.WriteString("\n\n")
		}
	}

	if len(req.ExternalSkills) > 0 {
		for _, id := range req.ExternalSkills {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			b.WriteString("## 外挂 SKILL: ")
			b.WriteString(id)
			b.WriteByte('\n')
			doc, ok := externalPackDocument(id)
			if !ok {
				b.WriteString("（未找到 id；请核对能力目录中的 id）\n\n")
				continue
			}
			b.WriteString(doc)
			b.WriteString("\n\n")
		}
	}

	if len(req.BuiltinSkills) > 0 {
		for _, name := range req.BuiltinSkills {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			b.WriteString("## 内置技能: ")
			b.WriteString(name)
			b.WriteByte('\n')
			detail, err := skill.GlobalManager.GetSkillDetail(name)
			if err != nil {
				b.WriteString(fmt.Sprintf("（未找到或未在 abilities.yml 启用: %v）\n\n", err))
				continue
			}
			raw, err := json.MarshalIndent(detail, "", "  ")
			if err != nil {
				b.WriteString(fmt.Sprintf("（序列化失败: %v）\n\n", err))
				continue
			}
			if inst := detail.Instance; inst != nil {
				b.WriteString("运行时描述: ")
				b.WriteString(strings.TrimSpace(inst.Description()))
				b.WriteByte('\n')
			}
			b.Write(raw)
			b.WriteString("\n\n")
		}
	}

	out := strings.TrimSpace(b.String())
	if out == "" {
		return "（无有效条目）", nil
	}
	return out, nil
}
