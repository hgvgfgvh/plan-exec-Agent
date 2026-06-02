package interaction

import "context"

// TurnProcessor 在已 BeginTurn 的 ctx 上执行 Agent 主链（由 portal 在 init 时注册）。
type TurnProcessor func(turnCtx context.Context, input, stagingID, routingPrefix string) error

var processTurn TurnProcessor

// SetProcessTurn 注册主链处理函数（portal 包 init 调用，避免 import cycle）。
func SetProcessTurn(fn TurnProcessor) {
	processTurn = fn
}

func invokeProcessTurn(turnCtx context.Context, input, stagingID, routingPrefix string) error {
	if processTurn == nil {
		return ErrProcessTurnNotRegistered
	}
	return processTurn(turnCtx, input, stagingID, routingPrefix)
}
