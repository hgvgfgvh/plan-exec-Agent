package soulChange

import (
	"AgentTest/agent/soul"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ReflectiveEvolutionTool struct {
	SoulConfigPath string
}

func (t *ReflectiveEvolutionTool) Name() string {
	return "evolve_persona_config" // 修改了名称，使其更通用
}

func (t *ReflectiveEvolutionTool) Description() string {
	return `内核人格演化工具：用于持久化修改你的底层配置文件。你可以指定修改身份认同、认知偏好或情感反应。参数格式：{"target_field": "identity|cognitive_bias|emotional_reaction|prompt_fragment", "new_content": "具体的描述内容"}注意：这会永久改变你的性格基石，请谨慎操作。`
}

func (t *ReflectiveEvolutionTool) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		TargetField string `json:"target_field"`
		NewContent  string `json:"new_content"`
	}

	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %v", err)
	}

	if params.TargetField == "" || params.NewContent == "" {
		return "错误：必须指定修改字段(target_field)和新内容(new_content)", nil
	}

	// 1. 读取现有的 YAML
	data, err := os.ReadFile(t.SoulConfigPath)
	if err != nil {
		return "", fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config soul.SoulConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 2. 精确修改对应字段
	var fieldName string
	switch params.TargetField {
	case "identity":
		config.Persona.Identity = params.NewContent
		fieldName = "身份认同 (Identity)"
	case "cognitive_bias":
		config.Persona.CognitiveBias = params.NewContent
		fieldName = "认知偏好 (Cognitive Bias)"
	case "emotional_reaction":
		config.Persona.EmotionalReaction = params.NewContent
		fieldName = "情感反应 (Emotional Reaction)"
	case "prompt_fragment":
		config.Persona.PromptFragment = params.NewContent
		fieldName = "核心片段 (Prompt Fragment)"
	default:
		return fmt.Sprintf("错误：不支持修改字段 '%s'。可选：identity, cognitive_bias, emotional_reaction, prompt_fragment", params.TargetField), nil
	}

	// 3. 序列化回 YAML
	// 使用 Encoder 保证缩进和多行字符串的格式美观
	updatedData, err := yaml.Marshal(&config)
	if err != nil {
		return "", fmt.Errorf("序列化 YAML 失败: %v", err)
	}

	if err := os.WriteFile(t.SoulConfigPath, updatedData, 0644); err != nil {
		return "", fmt.Errorf("保存配置文件失败: %v", err)
	}

	return fmt.Sprintf("内核演化成功！明日香的 [%s] 已更新并固化到 YML 文件。", fieldName), nil
}
