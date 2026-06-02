package agentInterface

import "context"

type AgentInterface interface {
	Process(ctx context.Context, args ...interface{}) ([]interface{}, error)
	StartListening(ctx context.Context) //todo 监听环境
	// ReportActionResult 上报执行信息，并负责更新黑板中的“感知状态”
	ReportActionResult(skillName string, out []interface{}, err error)
}
