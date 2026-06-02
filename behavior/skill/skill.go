package skill

import (
	"context"
)

// Skill 接口定义：支持多模态可变参数
type Skill interface {
	Name() string
	Description() string
	// Execute 接受变长参数，返回结果切片和错误
	Execute(ctx context.Context, args ...interface{}) ([]interface{}, error)
}

// SkillInfo 技能节点：对应 YAML 中的 skills 列表项
type SkillInfo struct {
	Name         string                 `yaml:"name"`        // 匹配代码中 Skill.Name()
	Description  string                 `yaml:"description"` // 具体的技能描述
	InputSchema  map[string]interface{} `yaml:"input_schema"`
	OutputSchema map[string]interface{} `yaml:"output_schema"` // 输出结果定义

	// Instance 指向具体的接口实现，不参与 YAML 序列化
	Instance Skill `yaml:"-"`
}

// Ability 能力节点：对应 YAML 中的 abilities 列表项
type Ability struct {
	Name        string       `yaml:"name"`        // 比如 "Open_URL"
	Description string       `yaml:"description"` // 比如 "调用搜索网页API"
	Skills      []*SkillInfo `yaml:"skills"`      // 该能力下的具体 Skill 实现
}

// Domain 领域节点：对应 YAML 中的 domains 列表项（最外层）
type Domain struct {
	Name        string     `yaml:"name"`        // 比如 "Browser_Domain"
	Description string     `yaml:"description"` // 比如 "信息搜索"
	Abilities   []*Ability `yaml:"abilities"`   // 领域下的能力目录
}
type NodeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
