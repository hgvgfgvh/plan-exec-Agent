package behaveFunc

import (
	"AgentTest/behavior/skill"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

type CerebellumDiscoveryTool struct{}

func (o CerebellumDiscoveryTool) Name() string {
	return "skill_hierarchy_discovery"
}

func (o CerebellumDiscoveryTool) Description() string {
	return `用于探索智能体可用的技能目录。支持分层探测：
1. 不传参数：返回所有 Domain 列表。
2. 传入 {"path": ["DomainName"]}：返回该领域下的 Ability 列表。
3. 传入 {"path": ["DomainName", "AbilityName"]}：返回该能力下的所有具体 Skill 及其详细输入输出参数(Schema)。
4. 特殊需求 {"path": ["SKILL", "skillName1","skillName2"......]}：第一个参数指定SKILL 后续传入具体的skillName就能直接获取这些skillName的详细输入输出参数。
建议逐层探测以节省 Token。`
}

func (o CerebellumDiscoveryTool) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Path []string `json:"path"`
	}

	// 解析输入
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		if input != "" && !strings.Contains(input, "{") {
			params.Path = []string{strings.Trim(input, "\" ")}
		}
	}

	// 1. 获取名称列表
	names, err := skill.GlobalManager.ListNodes(params.Path...)
	if err != nil {
		return "", fmt.Errorf("discovery failed: %v", err)
	}

	// 2. 核心逻辑判断：如果 path 长度为 2，说明现在返回的是具体 Skill 名单
	// 我们需要把这些名字转换成带有 Schema 的详细信息
	if len(params.Path) == 2 {
		var detailedSkills []*skill.SkillInfo
		for _, name := range names {
			detail, err := skill.GlobalManager.GetSkillDetail(name)
			if err == nil {
				detailedSkills = append(detailedSkills, detail)
			}
		}
		// 返回包含详细 Schema 的 JSON
		return o.marshalResult(detailedSkills)
	}

	var nodeInfo []*skill.NodeInfo
	for _, name := range names {
		description, err := skill.GlobalManager.GetNodeDescription(name)
		if err == nil {
			nodeInfo = append(nodeInfo, description)
		}
	}
	// 返回包含详细 Schema 的 JSON
	return o.marshalResult(nodeInfo)

}

// 辅助方法：统一序列化
func (o CerebellumDiscoveryTool) marshalResult(data interface{}) (string, error) {
	resultBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal error: %v", err)
	}
	return string(resultBytes), nil
}

func CreateCerebellumDiscoveryTool() tools.Tool {
	return CerebellumDiscoveryTool{}
}
