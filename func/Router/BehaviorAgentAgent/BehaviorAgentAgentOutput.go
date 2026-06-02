package BehaviorAgentAgent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"AgentTest/util/sendTopic"
	"context"
	"encoding/json"
	"fmt"
)

type BehaviorAgentAgentOutput struct{}

func (o BehaviorAgentAgentOutput) Name() string {
	return "BehaviorAgentAgentOutput"
}

func (o BehaviorAgentAgentOutput) Description() string {
	return "将最终执行结果投递给用户可见通道（门面输出）。输入应为一个包含 \"data\" 的 JSON 字符串例如：{\"data\": \"执行情况总结\"}"
}

func (o BehaviorAgentAgentOutput) Call(ctx context.Context, input string) (string, error) {
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
			return "", fmt.Errorf("BehaviorAgentAgentOutput 输入参数解析失败: %v", err)
		}
	}
	turnID, hop := runcontrol.TurnMetaFromContext(ctx)
	data := sendTopic.SanitizeFacadeText(params.Data)
	blackboard.GetInstance().PublishMsg(blackboard.TopicBehaviorOutput, data, turnID, hop)
	if !runcontrol.IsPlanStepExecution(ctx) && !runcontrol.IsPlanOrchestrating() {
		sendTopic.PublishFacadeDedup(turnID, data)
	}
	fmt.Printf("行为agent已通过门面投递: %s (turn=%s hop=%d)\n", params.Data, turnID, hop)
	return "行为agent数据已投递至用户门面", nil
}
