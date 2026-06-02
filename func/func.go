package _func

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tmc/langchaingo/tools"
)

// 1. 定义一个结构体来实现 tools.Tool 接口
type OrderStatusTool struct{}

// 2. 实现 Name 方法：返回工具的名称
func (o OrderStatusTool) Name() string {
	return "get_order_status"
}

// 3. 实现 Description 方法：告诉 Agent 什么时候该用这个工具
func (o OrderStatusTool) Description() string {
	return "根据订单ID查询订单的物流和发货状态。输入应为一个包含 order_id 的 JSON 字符串，例如：{\"order_id\": \"12345\"}"
}

// 4. 实现 Call 方法：这是真正的执行逻辑
// 注意：如果你的版本中接口定义是 Execute，请将方法名改为 Execute
func (o OrderStatusTool) Call(ctx context.Context, input string) (string, error) {
	// 这里的 input 是模型生成的参数字符串
	var params struct {
		OrderID string `json:"order_id"`
	}

	// 尝试解析 JSON
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 容错处理：如果模型直接传了字符串而不是 JSON，尝试直接使用
		if input != "" {
			params.OrderID = input
		} else {
			return "", fmt.Errorf("无效的输入格式: %v", err)
		}
	}

	// 模拟数据库查询逻辑
	if params.OrderID == "12345" {
		return "订单状态：已发货，预计明日送达AAA。", nil
	}
	return "未找到该订单信息AAA。", nil
}

// 5. 修正 CreateOrderTool 函数
func CreateOrderTool() tools.Tool {
	// 返回结构体实例，因为它实现了 tools.Tool 接口
	return OrderStatusTool{}
}
