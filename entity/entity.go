package entity

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

type ModeInterface interface {
	GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error)
}

type ONNXModel interface {
	Chat(input string) (string, error)
}
