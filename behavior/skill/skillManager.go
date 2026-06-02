package skill

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3" // 需要安装: go get gopkg.in/yaml.v3
)

// ConfigWrapper 用于解析 YAML 的最外层结构
type ConfigWrapper struct {
	Domains []*Domain `yaml:"domains"`
}

type SkillManager struct {
	Registry map[string]Skill
	Domains  map[string]*Domain
	mu       sync.RWMutex
}

var GlobalManager = &SkillManager{
	Registry: make(map[string]Skill),
	Domains:  make(map[string]*Domain),
}

// Regist 供具体 Skill 注册
func (sm *SkillManager) Regist(skill Skill) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.Registry[skill.Name()] = skill
	fmt.Printf("📦 Skill [ %s ] 已注册到系统\n", skill.Name())
}

// HasRegisteredSkill 判断内置 SKILL 是否已在代码中注册。
func (sm *SkillManager) HasRegisteredSkill(name string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.Registry[name]
	return ok
}

// LoadConfig 从外部加载 YAML 配置文件并完成绑定
func (sm *SkillManager) LoadConfig(filePath string) error {
	// 1. 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read config file error: %v", err)
	}

	// 2. 解析 YAML
	var config ConfigWrapper
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("unmarshal yaml error: %v", err)
	}

	// 3. 写入管理器并执行绑定
	sm.mu.Lock()
	sm.Domains = make(map[string]*Domain) // 清空旧配置
	for _, d := range config.Domains {
		sm.Domains[d.Name] = d
	}
	sm.mu.Unlock()

	// 4. 调用你写的 InitLinks 将 Instance 填入
	sm.InitLinks()

	fmt.Printf("🚀 配置加载完成，共加载 %d 个领域\n", len(sm.Domains))
	return nil
}

// InitLinks 建立配置文件与代码实例的映射
func (sm *SkillManager) InitLinks() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, domain := range sm.Domains {
		for _, ability := range domain.Abilities {
			for _, skillInfo := range ability.Skills {
				if instance, ok := sm.Registry[skillInfo.Name]; ok {
					skillInfo.Instance = instance
				} else {
					fmt.Printf("⚠️ 警告: 配置文件中的技能 [%s] 未在代码中注册实现\n", skillInfo.Name)
				}
			}
		}
	}
}

// ListNodes 灵活探测层级内容
// args 为空：返回所有 Domain Name
// args 为 [domain]：返回该 Domain 下所有 Ability Name
// args 为 [domain, ability]：返回该 Ability 下所有 Skill Name
// args 为 [SKILL,skillName1,skillName2...] 返回具体的skillName名称
func (sm *SkillManager) ListNodes(path ...string) ([]string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	level := len(path)

	// 1. 不传参数：列出所有领域
	if level == 0 {
		var names []string
		for name := range sm.Domains {
			names = append(names, name)
		}
		return names, nil
	}

	// --- 特殊处理: 直接匹配 Skill ---
	if path[0] == "SKILL" {
		// 使用 map 去重，防止一个技能匹配了多个关键词导致重复返回
		uniqueNames := make(map[string]struct{})
		keywords := path[1:] // 获取 "SKILL" 之后的所有参数

		for name := range sm.Registry {
			for _, kw := range keywords {
				if name == kw {
					uniqueNames[name] = struct{}{}
					break // 只要匹配中一个关键字，就跳出当前技能的关键字循环
				}
			}
		}

		// 转换为切片返回
		var result []string
		for name := range uniqueNames {
			result = append(result, name)
		}
		return result, nil
	}

	// 2. 传入领域：列出该领域下的所有能力
	domainName := path[0]
	domain, ok := sm.Domains[domainName]
	if !ok {
		return nil, fmt.Errorf("未找到领域: %s", domainName)
	}

	if level == 1 {
		var names []string
		for _, abi := range domain.Abilities {
			names = append(names, abi.Name)
		}
		return names, nil
	}

	// 3. 传入领域+能力：列出该能力下的所有技能
	abilityName := path[1]
	if level == 2 {
		for _, abi := range domain.Abilities {
			if abi.Name == abilityName {
				var names []string
				for _, s := range abi.Skills {
					names = append(names, s.Name)
				}
				return names, nil
			}
		}
		return nil, fmt.Errorf("在领域 %s 下未找到能力: %s", domainName, abilityName)
	}

	return nil, fmt.Errorf("参数过多，最高支持 (domain, ability)")
}

// Search 直接根据 skillName 获取技能实例
func (sm *SkillManager) Search(skillName string) (Skill, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 1. 直接从注册表中获取实例
	inst, ok := sm.Registry[skillName]
	if !ok {
		return nil, fmt.Errorf("skill implementation not registered: %s", skillName)
	}

	// 2. 可选校验：检查该技能是否在当前的 YAML 域名结构中激活（Instance 是否已绑定）
	// 这一步确保了即使代码里有实现，如果 YAML 配置里没开放，Agent 也调不到。
	isConfigured := false
	for _, domain := range sm.Domains {
		for _, abi := range domain.Abilities {
			for _, sInfo := range abi.Skills {
				if sInfo.Name == skillName && sInfo.Instance != nil {
					isConfigured = true
					break
				}
			}
		}
	}

	if !isConfigured {
		return nil, fmt.Errorf("skill %s is registered but not enabled in YAML config", skillName)
	}

	return inst, nil
}

// GetSkillDetail 获取指定技能的详细定义（给大脑看）
func (sm *SkillManager) GetSkillDetail(skillName string) (*SkillInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, domain := range sm.Domains {
		for _, abi := range domain.Abilities {
			for _, sInfo := range abi.Skills {
				if sInfo.Name == skillName {
					return sInfo, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("skill detail not found: %s", skillName)
}

// GetNodeDescription 获取指定节点（Domain 或 Ability）的描述信息
func (sm *SkillManager) GetNodeDescription(nodeName string) (*NodeInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 1. 先尝试在 Domain 层级查找
	if domain, ok := sm.Domains[nodeName]; ok {
		return &NodeInfo{
			Name:        domain.Name,
			Description: domain.Description,
		}, nil
	}

	// 2. 如果没找到，深入到 Ability 层级查找
	for _, domain := range sm.Domains {
		for _, abi := range domain.Abilities {
			if abi.Name == nodeName {
				return &NodeInfo{
					Name:        abi.Name,
					Description: abi.Description,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("node description not found: %s", nodeName)
}

// CatalogSnapshot 返回 abilities 配置树的只读快照，供 Agent 能力目录（第一层）生成使用。
func (sm *SkillManager) CatalogSnapshot() []*Domain {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	out := make([]*Domain, 0, len(sm.Domains))
	for _, d := range sm.Domains {
		out = append(out, d)
	}
	return out
}
