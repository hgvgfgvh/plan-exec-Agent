package active

import (
	"AgentTest/behavior/skill"
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/go-vgo/robotgo"
)

type SuperKeySkill struct{}

func (s *SuperKeySkill) Name() string { return "SuperKeyCombine" }
func (s *SuperKeySkill) Description() string {
	return "执行组合快捷键，支持 win, control, alt, shift 等"
}

func (s *SuperKeySkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 上下文预检
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("SuperKey 需要传入按键列表")
	}

	var rawKeys []string
	// 兼容处理 AI 传来的 []interface{} 格式
	if list, ok := args[0].([]interface{}); ok {
		for _, k := range list {
			if str, ok := k.(string); ok {
				rawKeys = append(rawKeys, str)
			}
		}
	} else if singleKey, ok := args[0].(string); ok {
		// 容错：如果 AI 只传了一个字符串而不是数组
		rawKeys = append(rawKeys, singleKey)
	}

	if len(rawKeys) == 0 {
		return nil, fmt.Errorf("未识别到有效的按键字符")
	}

	// 2. 核心逻辑：键名内部转发与映射
	finalKeys := make([]string, len(rawKeys))
	for i, k := range rawKeys {
		lowerKey := strings.ToLower(k)

		// A. 将 win/windows 统一映射为 robotgo 识别的 command
		if lowerKey == "win" || lowerKey == "windows" {
			finalKeys[i] = "command"
			continue
		}

		// B. Mac 平台自动将 control 映射为 command (符合苹果用户习惯)
		if lowerKey == "control" && runtime.GOOS == "darwin" {
			finalKeys[i] = "command"
			continue
		}

		// C. 其他按键保持原样
		finalKeys[i] = lowerKey
	}

	fmt.Printf("⌨️  执行物理快捷键: 原始输入%v -> 实际映射%v\n", rawKeys, finalKeys)

	// 3. 执行物理动作：顺序按下
	for _, k := range finalKeys {
		robotgo.KeyDown(k)
		robotgo.MilliSleep(50) // 增加物理延迟，确保系统内核捕获到修饰键状态
	}

	// 关键停顿：确保组合键在系统层级生效
	robotgo.MilliSleep(100)

	// 4. 执行物理动作：逆序释放 (LIFO 栈式释放)
	// 这是防止“按键粘连”的关键：先按下的最后松开

	for i := len(finalKeys) - 1; i >= 0; i-- {
		robotgo.KeyUp(finalKeys[i])
		robotgo.MilliSleep(20)
	}

	return []interface{}{"success"}, nil
}

func init() {
	skill.GlobalManager.Regist(&SuperKeySkill{})
}
