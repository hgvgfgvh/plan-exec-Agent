package executor

import (
	"AgentTest/behavior/skill"
	"fmt"
	"strconv"
	"strings"
)

// Node 定义多叉树节点
type SkillNode struct {
	Skill    skill.Skill  // 具体的技能实现
	Children []*SkillNode // 子技能节点
}

// SkillStep 是供 Agent / 外部 RPC 使用的「结构化技能树」DSL。
//
// 设计目的：替换原 "Name:childCount,Name:childCount,..." 字符串语法。
// 优势：
//   - 拼写错误（漏逗号、子节点数算错）会在 JSON 解析或 BuildSkillTree 校验阶段被直接拒绝；
//   - LLM/前端可生成任意嵌套深度而不必关心展开顺序；
//   - 旧字符串解析 ParseSkillTree 仍然保留作为兼容路径，无需一次性迁移所有 prompt。
//
// 序列化示例（单层）：
//
//	{"skill":"PowerShell"}
//
// 嵌套示例（Generate_Word_Report 是根，先跑 PowerShell 收集再生成）：
//
//	{
//	  "skill":"Generate_Word_Report",
//	  "children":[
//	    {"skill":"PowerShell"}
//	  ]
//	}
type SkillStep struct {
	Skill    string       `json:"skill"`
	Children []*SkillStep `json:"children,omitempty"`
}

// BuildSkillTree 把结构化的 SkillStep 解析为可执行的 SkillNode 树。
// 校验：每个 skill 名必须存在于 skill.GlobalManager.Registry，否则报错并指明完整路径，
// 帮助 Agent 对照能力目录或 get_capability_details 重新核对技能名，而不是把错误埋到执行期。
func BuildSkillTree(step *SkillStep) (*SkillNode, error) {
	return buildSkillTree(step, "")
}

func buildSkillTree(step *SkillStep, path string) (*SkillNode, error) {
	if step == nil {
		return nil, fmt.Errorf("skill step is nil at path %q", path)
	}
	name := strings.TrimSpace(step.Skill)
	if name == "" {
		return nil, fmt.Errorf("skill name is empty at path %q", path)
	}
	here := name
	if path != "" {
		here = path + "/" + name
	}
	instance, ok := skill.GlobalManager.Registry[name]
	if !ok {
		return nil, fmt.Errorf("skill not registered: %q (path=%s)；请对照 Agent 能力目录或 get_capability_details 确认内置技能名", name, here)
	}
	node := &SkillNode{
		Skill:    instance,
		Children: make([]*SkillNode, 0, len(step.Children)),
	}
	for i, child := range step.Children {
		childPath := fmt.Sprintf("%s[%d]", here, i)
		built, err := buildSkillTree(child, childPath)
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, built)
	}
	return node, nil
}

// ParseSkillTree 将字符串解析为 Skill 树（兼容路径，新代码请优先使用 BuildSkillTree）。
func ParseSkillTree(input string) (*SkillNode, error) {

	// 1. 预处理
	tokens := strings.Split(strings.ReplaceAll(input, " ", ""), ",")
	index := 0

	// 2. 递归构建函数
	var build func() (*SkillNode, error)
	build = func() (*SkillNode, error) {
		if index >= len(tokens) {
			return nil, nil
		}

		// 解析标记，例如 "FindUser:2"
		parts := strings.Split(tokens[index], ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid token format: %s", tokens[index])
		}

		skillName := parts[0]
		childCount, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid child count in: %s", tokens[index])
		}
		index++

		instance, exists := skill.GlobalManager.Registry[skillName]

		if !exists {
			return nil, fmt.Errorf("skill not found in registry: %s", skillName)
		}

		// 4. 创建包装节点
		node := &SkillNode{
			Skill:    instance,
			Children: make([]*SkillNode, 0, childCount),
		}

		// 5. 递归处理子节点
		for i := 0; i < childCount; i++ {
			child, err := build()
			if err != nil {
				return nil, err
			}
			if child != nil {
				node.Children = append(node.Children, child)
			}
		}

		return node, nil
	}

	return build()
}

// PrintTree 只要传入解析后的树根节点即可
func PrintTree(node *SkillNode) {
	if node == nil {
		fmt.Println("Tree is empty")
		return
	}
	// 调用内部递归函数，初始深度为 0
	printRecursive(node, 0)
}

// printRecursive 内部递归逻辑
func printRecursive(node *SkillNode, depth int) {
	// 1. 生成缩进：每层深度增加 4 个空格
	indent := strings.Repeat("    ", depth)

	// 2. 打印当前节点
	// 如果 Skill 接口还没实现 Name()，可以先打印指针或固定占位符
	name := "UnknownSkill"
	if node.Skill != nil {
		name = node.Skill.Name()
	}

	fmt.Printf("%s└── %s\n", indent, name)

	// 3. 递归打印所有子节点
	for _, child := range node.Children {
		printRecursive(child, depth+1)
	}
}
