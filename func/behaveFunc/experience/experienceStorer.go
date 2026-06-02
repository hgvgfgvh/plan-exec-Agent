package experience

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExperienceStorer 定义内部接口，确保 Tool 可以调用 Manager 的存储功能
type ExperienceStorer interface {
	StoreExperience(ctx context.Context, query, skillTree string) error
}

// ExperienceStoreTool 适配存入经验的工具
type ExperienceStoreTool struct {
	Storer ExperienceStorer
}

func (t ExperienceStoreTool) Name() string {
	return "store_skill_experience" // 工具名称：存储技能经验
}

func (t ExperienceStoreTool) Description() string {
	return "按需主动触发!当一个任务成功完成，。将需求和对应的 调用过程中的经验总结 存入长时经验库，以便未来复用。输入参数包含需求描述和对应的 经验总结。例如：{\"query\": \"具体的需求描述\"，\"skill_tree\": \"经验总结\"}"
}

func (t ExperienceStoreTool) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Query     string `json:"query"`      // 原始需求
		SkillTree string `json:"skill_tree"` // 执行成功的 SkillTree 字符串
	}

	// 1. 解析输入
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "", fmt.Errorf("failed to parse storage input: %v, input: %s", err, input)
	}

	if params.Query == "" || params.SkillTree == "" {
		return "存储失败：需求描述或 SkillTree 不能为空", nil
	}

	// 2. 调用 Manager 写入 JSON 文件
	go t.Storer.StoreExperience(ctx, params.Query, params.SkillTree)
	if err != nil {
		return "", fmt.Errorf("storage persistence failed: %v", err)
	}

	// 3. 返回成功响应给 Agent
	return fmt.Sprintf("成功！已将需求 [%s] 的执行经验固化到长时库中。", params.Query), nil
}
