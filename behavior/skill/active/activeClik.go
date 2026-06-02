package active

import (
	"AgentTest/behavior/skill"
	"context"
	"fmt"
	"strings"

	"github.com/go-vgo/robotgo"
)

// ClickSkill 对应 YAML 中的 Click 技能：支持坐标定位与按键类型选择
type ClickSkill struct{}

func (s *ClickSkill) Name() string { return "Click" }
func (s *ClickSkill) Description() string {
	return "点击屏幕指定坐标并指定是左键还是右键点击"
}

func (s *ClickSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 上下文预检：确保指令在有效期内
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. 参数校验：必须满足 [x, y, way] 三个输入
	if len(args) < 3 {
		return nil, fmt.Errorf("Click 技能需要 x, y 和 way (left/right) 三个参数")
	}

	// 内部转换工具：处理 AI 传入的数值类型不确定性
	toInt := func(v interface{}) (int, bool) {
		switch val := v.(type) {
		case int:
			return val, true
		case float64: // JSON 映射常为 float64
			return int(val), true
		default:
			return 0, false
		}
	}

	x, okX := toInt(args[0])
	y, okY := toInt(args[1])
	way, okWay := args[2].(string)

	if !okX || !okY || !okWay {
		return nil, fmt.Errorf("参数格式错误：预期 [int, int, string]")
	}

	// 3. 逻辑预处理：标准化点击指令
	clickWay := strings.ToLower(way)
	if clickWay != "left" && clickWay != "right" {
		// 容错处理：默认回退至左键
		clickWay = "left"
	}

	// 4. 物理执行：调用 robotgo 驱动
	// 移动鼠标至目标点
	robotgo.Move(x, y)
	robotgo.MilliSleep(50) // 增加微小延迟模拟物理平滑度

	// 执行物理点击
	// robotgo.Click(button, doubleClick)
	robotgo.Click(clickWay, false)

	fmt.Printf("🖱️ 物理执行完成: 坐标 [%d, %d] 动作 [%s]\n", x, y, clickWay)

	// 5. 返回输出：符合 YAML 定义的 output_schema
	return []interface{}{"success"}, nil
}

func init() {
	// 注册技能至全局管理器
	skill.GlobalManager.Regist(&ClickSkill{})
}
