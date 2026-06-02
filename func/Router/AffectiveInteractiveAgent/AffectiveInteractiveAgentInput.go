package AffectiveInteractiveAgent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"context"
	"encoding/json"
	"fmt"
)

type AffectiveInteractiveAgentInput struct{}

func (o AffectiveInteractiveAgentInput) Name() string {
	return "AffectiveInteractiveAgentInput"
}

func (o AffectiveInteractiveAgentInput) Description() string {
	return "将【用户侧对话/解释类需求】交给情感直接交互 Agent（走主对话 Process）。输入 JSON：{\"data\":\"用户原话或需门面转述的完整信息\"}，不得删减用户语义。"
}

func (o AffectiveInteractiveAgentInput) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Data string `json:"data"`
	}

	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		if input != "" {
			params.Data = input
		} else {
			return "", fmt.Errorf("AffectiveInteractiveAgentInput 输入参数解析失败: %v", err)
		}
	}
	// 主路径：由丘脑路由投递给交互门面的「用户意图」，与脑区反馈 input 主题分流。
	// 携带回合元信息，供下游 Agent 在出站时继承 Hop。
	turnID, hop := runcontrol.TurnMetaFromContext(ctx)
	blackboard.GetInstance().PublishMsg(blackboard.TopicAffectiveDispatch, params.Data, turnID, hop)
	fmt.Printf("数据成功传递给情感直接交互 agent: %s (turn=%s hop=%d)\n", params.Data, turnID, hop)
	return "数据成功传递给情感直接交互agent", nil
}
