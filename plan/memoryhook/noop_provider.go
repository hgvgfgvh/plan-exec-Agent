package memoryhook

import "context"

// NoopProvider 不连接 Memory MCP，始终返回未命中（阶段一默认）。
type NoopProvider struct{}

func (NoopProvider) Name() string { return "noop" }

func (NoopProvider) Retrieve(context.Context, RetrieveRequest) (Experience, error) {
	return Experience{Matched: false}, nil
}
