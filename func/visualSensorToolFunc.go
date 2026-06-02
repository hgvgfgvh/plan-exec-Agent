package _func

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tmc/langchaingo/tools"
)

// VisualSensorTool 模拟视觉传感器
type VisualSensorTool struct{}

func (v VisualSensorTool) Name() string {
	return "get_visual_info"
}

func (v VisualSensorTool) Description() string {
	return "获取周围环境的视觉消息。输入应为包含 direction 的 JSON 字符串，direction 可选值为：'前'、'后'、'左'、'右'。例如：{\"direction\": \"前\"}"
}

func (v VisualSensorTool) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Direction string `json:"direction"`
	}

	// 解析输入
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 容错：如果模型直接输出了方向词
		params.Direction = input
	}

	// 模拟不同方向的视觉反馈
	switch params.Direction {
	case "前":
		return "前方检测到一扇木门和一个红色的消防栓。", nil
	case "后":
		return "后方是一个白色的书架，上面摆满了技术书籍。", nil
	case "左":
		return "左侧有一扇落地窗，外面正在下雨。", nil
	case "右":
		return "右侧是一个办公位，电脑屏幕亮着。", nil
	default:
		return fmt.Sprintf("无法观察方向 '%s'，请指定前、后、左、右。", params.Direction), nil
	}
}

func CreateVisualTool() tools.Tool {
	return VisualSensorTool{}
}
