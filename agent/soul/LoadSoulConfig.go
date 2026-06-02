package soul

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type SoulConfig struct {
	Persona struct {
		Name              string `yaml:"name"`
		Identity          string `yaml:"identity"`
		CognitiveBias     string `yaml:"cognitive_bias"`
		EmotionalReaction string `yaml:"emotional_reaction"`
		PromptFragment    string `yaml:"prompt_fragment"`
	} `yaml:"persona"`
}

// LoadSoulConfig 从 YAML 文件读取灵魂配置
func LoadSoulConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var config SoulConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", err
	}

	// 将 YAML 各项组合成一个完整的系统 Prompt 前缀
	soulPrompt := fmt.Sprintf(
		"你的身份: %s\n%s\n【认知逻辑】:\n%s\n【情感表现】:\n%s\n\n%s",
		config.Persona.Name,
		config.Persona.Identity,
		config.Persona.CognitiveBias,
		config.Persona.EmotionalReaction,
		config.Persona.PromptFragment,
	)
	return soulPrompt, nil
}
