package soulhook

import "context"

// Provider Soul MCP 插件：回合前 retrieve hints、回合后 store（经 DialogueStorer）。
type Provider interface {
	Name() string
	RetrieveHints(ctx context.Context, userInput string) string
}

// DialogueStorer 异步 soul_store 的 WebUI 对话材料写入。
type DialogueStorer interface {
	StoreDialogue(ctx context.Context, in DialogueStoreInput, content string) error
}

// DialogueStoreInput Host 传入的关联元数据（content 由 BuildWebUIDialogueContent 生成）。
type DialogueStoreInput struct {
	TurnID         string
	UserInput      string
	AssistantReply string
	ProcessError   string
}
