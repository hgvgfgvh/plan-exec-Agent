package active

import (
	"AgentTest/behavior/skill"
	"context"
	"fmt"
	"time"

	"github.com/go-vgo/robotgo"
)

// TypeStringSkill 对应 YAML 中的 TypeString 技能
type TypeStringSkill struct{}

func (s *TypeStringSkill) Name() string { return "TypeString" }

func (s *TypeStringSkill) Description() string {
	return "在当前光标处输入文本内容（支持中英文）。注意：执行前请确保目标输入框已获取焦点。"
}

func (s *TypeStringSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 立即检查上下文：防止在任务取消后继续物理操作
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. 参数提取与校验
	if len(args) < 1 {
		return nil, fmt.Errorf("TypeString 技能需要至少 1 个参数: A_text")
	}

	// 健壮的字符串转换逻辑
	var textToType string
	switch v := args[0].(type) {
	case string:
		textToType = v
	case fmt.Stringer:
		textToType = v.String()
	default:
		// 容错处理：即使 AI 传了数字，也转为字符串输入
		textToType = fmt.Sprintf("%v", v)
	}

	if textToType == "" {
		return []interface{}{"fail"}, fmt.Errorf("输入内容不能为空")
	}

	// 3. 执行物理模拟
	fmt.Printf("⌨️  执行键盘输入任务: [%s]\n", textToType)

	// 物理模拟前微小停顿，确保系统事件队列准备就绪
	time.Sleep(50 * time.Millisecond)

	// 使用 robotgo 输入字符串
	// 注意：在 Windows 环境下，TypeStr 性能受输入法影响，建议提醒用户切换至英文状态
	robotgo.TypeStr(textToType)

	// 4. 输入后置缓冲，防止后续动作过快导致粘连
	time.Sleep(100 * time.Millisecond)

	return []interface{}{"success"}, nil
}

func init() {
	// 将技能注册到全局管理器
	skill.GlobalManager.Regist(&TypeStringSkill{})
}
