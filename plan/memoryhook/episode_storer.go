package memoryhook

import "context"

// EpisodeStoreInput 本轮 episode 写入 Memory MCP 的材料（Host 拼装，字符串协议）。
type EpisodeStoreInput struct {
	TurnID          string
	UserRequirement string
	AssistantReply  string
	ProcessError    string
	PlanDocumentID  string // 可选，从门户回复 footer 解析
}

// EpisodeStorer 可选插件能力：支持 memory_store 的 Provider 实现（如 mcp）。
type EpisodeStorer interface {
	StoreEpisode(ctx context.Context, in EpisodeStoreInput, content string) error
}
