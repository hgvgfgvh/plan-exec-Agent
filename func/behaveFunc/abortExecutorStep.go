package behaveFunc

import (
	"AgentTest/body/blackboard"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/tools"
)

type AbortExecutorStep struct{}

func (o AbortExecutorStep) Name() string {
	return "AbortExecutorStep"
}

func (o AbortExecutorStep) Description() string {
	return "立即中止当前正在执行的技能树任务。当发现环境异常或任务不再需要时使用。输入应为一个包含 \"because\" 的 JSON 字符串例如：{\"because\": \"你下发停止的原因是什么？基于什么信息\"}"
}

func (o AbortExecutorStep) Call(ctx context.Context, input string) (string, error) {
	var params struct {
		because string `json:"because"`
	}

	// 解析 Agent 传来的参数
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 简单的容错处理
		if input != "" {
			params.because = input
		} else {
			return "", fmt.Errorf("speech_by_pc 输入参数解析失败: %v", err)
		}
	}
	// 向黑板发布取消信号
	blackboard.GetInstance().Publish(blackboard.TopicAgentAbort, "User or System requested abortion")
	return "中止指令已下达" + params.because, nil
}

func CreateAbortExecutorStep() tools.Tool {
	return AbortExecutorStep{}
}
