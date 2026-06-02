package facadeInteraction

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/util/sendTopic"
	"context"
	"encoding/json"
	"fmt"
)

type FacadeInteractionOutPut struct{}

func (o FacadeInteractionOutPut) Name() string {
	return "FacadeInteractionOutPut"
}

func (o FacadeInteractionOutPut) Description() string {
	return "回复消息给用户（注意！【路由中心传递来的信息】的形式是异步的，你在总结处理后，需要通过这个方法回复给用户）。输入应为一个包含 \"data\" 的 JSON 字符串例如：{\"data\": \"具体的内容\"}"
}

func (o FacadeInteractionOutPut) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Data string `json:"data"`
	}

	// 解析 Agent 传来的参数
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 简单的容错处理
		if input != "" {
			params.Data = input
		} else {
			return "", fmt.Errorf("FacadeInteractionOutPut 输入参数解析失败: %v", err)
		}
	}

	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	sendTopic.PublishFacadeDedup(turnID, params.Data)
	fmt.Printf("门面数据输出%s\n", params.Data)
	return "门面数据输出", nil
}
