package memoryhook

import (
	"AgentTest/plan/todolist"
	"context"
)

// RetrieveRequest 传给经验 Provider 的检索上下文。
type RetrieveRequest struct {
	UserRequirement string
	Document        *todolist.Document
}

// Provider 经验检索插件：由 noop / 自定义 / 未来 Memory MCP 客户端实现。
type Provider interface {
	Name() string
	Retrieve(ctx context.Context, req RetrieveRequest) (Experience, error)
}
