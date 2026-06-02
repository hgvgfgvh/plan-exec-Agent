package active

import (
	"AgentTest/behavior/skill"
	"context"
	"fmt"

	"github.com/go-vgo/robotgo"
)

type DragSkill struct{}

func (s *DragSkill) Name() string        { return "DragAndDrop" }
func (s *DragSkill) Description() string { return "模拟真人平滑拖拽动作" }

func (s *DragSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("DragAndDrop 需要 4 个参数: startX, startY, endX, endY")
	}

	// 这里使用你 demo 中的逻辑
	toInt := func(v interface{}) int {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		default:
			return 0
		}
	}

	x1, y1 := toInt(args[0]), toInt(args[1])
	x2, y2 := toInt(args[2]), toInt(args[3])

	fmt.Printf("🖱️ 执行平滑拖拽: 从 [%d,%d] 到 [%d,%d]\n", x1, y1, x2, y2)

	// 物理操作
	robotgo.MoveSmooth(x1, y1)
	robotgo.MilliSleep(200)
	robotgo.MouseDown("left")
	robotgo.MilliSleep(100)
	robotgo.MoveSmooth(x2, y2, 1.0, 15.0) // 模拟真人轨迹
	robotgo.MilliSleep(200)
	robotgo.MouseUp("left")

	return []interface{}{"success"}, nil
}

func init() {
	skill.GlobalManager.Regist(&DragSkill{})
}
