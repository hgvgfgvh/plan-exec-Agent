package BehaviorAgentAgent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"context"
	"encoding/json"
	"fmt"
)

type BehaviorAgentAgentInput struct{}

func (o BehaviorAgentAgentInput) Name() string {
	return "BehaviorAgentAgentInput"
}

func (o BehaviorAgentAgentInput) Description() string {
	return "将数据路由传递给行为agent。输入应为一个包含 \"data\" 的 JSON 字符串例如：{\"data\": \"具体的行为指令\"}"
}

func (o BehaviorAgentAgentInput) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Data string `json:"data"`
	}

	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		if input != "" {
			params.Data = input
		} else {
			return "", fmt.Errorf("BehaviorAgentAgentInput 输入参数解析失败: %v", err)
		}
	}
	// 携带回合元信息发布；由 Router 反思发起的转发已在 Router 内将 ctx.Hop 增至 nextHop。
	turnID, hop := runcontrol.TurnMetaFromContext(ctx)
	blackboard.GetInstance().PublishMsg(blackboard.TopicBehaviorInput, params.Data, turnID, hop)
	fmt.Printf("数据成功传递给行为agent: %s (turn=%s hop=%d)\n", params.Data, turnID, hop)
	return "数据成功传递给行为agent", nil
}
