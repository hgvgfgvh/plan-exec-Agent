package experience

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExperienceSearcher 定义内部接口，确保 Tool 可以调用 Manager 的检索功能
type ExperienceSearcher interface {
	RetrieveExperience(ctx context.Context, query string) (string, error)
}

// ExperienceRetrieveTool 适配 langchaingo 的工具接口
type ExperienceRetrieveTool struct {
	Searcher ExperienceSearcher
}

func (t ExperienceRetrieveTool) Name() string {
	return "recall_skill_experience" // 工具名称：回想技能经验
}

func (t ExperienceRetrieveTool) Description() string {
	return "当你需要根据当前需求寻找过去成功执行过的 Skill经验时使用。输入是需求关键词。返回是 成功的调用的经验总结，可用于参考。例如：{\"query\": \"具体的需求描述\"}"
}

func (t ExperienceRetrieveTool) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Query string `json:"query"`
	}

	// 1. 鲁棒的输入解析
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 容错：如果模型直接传入了纯字符串
		if input != "" {
			params.Query = input
		} else {
			return "", fmt.Errorf("invalid input format: %v", err)
		}
	}

	// 2. 调用 FileExperienceManager 进行文件检索
	skillTree, err := t.Searcher.RetrieveExperience(ctx, params.Query)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve experience: %v", err)
	}

	// 3. 结果处理
	if skillTree == "" {
		return "在经验库中未找到匹配的成功案例。请尝试按需构建新的 SkillTree。", nil
	}

	// 4. 返回格式化结果，告知模型这可以直接使用
	return fmt.Sprintf("检索到匹配该需求的成功经验，SkillTree 如下 仅供参考：\n%s", skillTree), nil
}
