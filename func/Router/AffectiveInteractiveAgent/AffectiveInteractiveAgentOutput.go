package AffectiveInteractiveAgent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"context"
	"encoding/json"
	"fmt"
)

// AffectiveInteractiveAgentOutput 情感 Agent 的输出适配器
type AffectiveInteractiveAgentOutput struct{}

func (o AffectiveInteractiveAgentOutput) Name() string {
	return "AffectiveInteractiveAgentOutput"
}

func (o AffectiveInteractiveAgentOutput) Description() string {
	return "核心路由工具：将无法通过直接对话交互完成的任务（比如行为编排，文件管理等这类需要具体执行的任务），提交给路由中心 进行调度完成。输入应为一个包含 \"data\" 的 JSON 字符串，例如：{\"data\": \"打开文件夹\"}"
}

func (o AffectiveInteractiveAgentOutput) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		Data string `json:"data"`
	}

	// 1. 解析输入参数
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 容错处理：如果不是标准的 JSON，尝试直接使用原始字符串
		if input != "" {
			params.Data = input
		} else {
			return "", fmt.Errorf("AffectiveInteractiveAgentOutput 输入参数解析失败: %v", err)
		}
	}

	// 2. 向黑板发布信号（携带回合元信息，供 Router 反思跳数预算使用）
	// 路由 Agent（Router）会订阅这个主题并进行二次分发
	turnID, hop := runcontrol.TurnMetaFromContext(ctx)
	blackboard.GetInstance().PublishMsg(blackboard.TopicAffectiveOutput, params.Data, turnID, hop)
	fmt.Printf("情感直接交互agent,提交路由中心已完成: %s (turn=%s hop=%d)\n", params.Data, turnID, hop)
	return "情感直接交互agent,提交路由中心已完成", nil
}
