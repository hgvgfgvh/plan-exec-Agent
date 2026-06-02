package dialogueHistoryArchiverManager

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
)

type DialogueHistoryArchiverManager struct {
	Model            llms.Model
	TokenLimit       int
	KeepLastRounds   int
	SummaryPrefix    string
	ReservedResponse int
}

// 创建
func NewDialogueHistoryArchiverManager(
	model llms.Model,
	tokenLimit int,
	keepLastRounds int,
) *DialogueHistoryArchiverManager {

	return &DialogueHistoryArchiverManager{
		Model:            model,
		TokenLimit:       tokenLimit,
		KeepLastRounds:   keepLastRounds,
		SummaryPrefix:    "【历史对话摘要】\n",
		ReservedResponse: 500,
	}
}
func (d *DialogueHistoryArchiverManager) MaybeArchive(
	ctx context.Context,
	mem *memory.ConversationBuffer,
) error {

	allMessages, err := mem.ChatHistory.Messages(ctx)
	if err != nil {
		return err
	}

	// 拼接为文本计算token
	var builder strings.Builder
	for _, m := range allMessages {
		builder.WriteString(m.GetContent())
		builder.WriteString("\n")
	}

	currentTokens := estimateTokens(builder.String())

	if currentTokens < d.TokenLimit-d.ReservedResponse {
		return nil // 不触发
	}

	fmt.Printf(">>> [Archiver] Token超限，开始压缩...\n")

	// 计算需要压缩的范围
	totalRounds := len(allMessages)
	keepCount := d.KeepLastRounds * 2
	if totalRounds <= keepCount {
		return nil
	}

	splitIndex := totalRounds - keepCount
	toSummarize := allMessages[:splitIndex]
	toKeep := allMessages[splitIndex:]

	// 拼接需要压缩的内容
	var summarizeText strings.Builder
	for _, m := range toSummarize {
		role := "Human"
		if m.GetType() == llms.ChatMessageTypeAI {
			role = "AI"
		}
		summarizeText.WriteString(role + ": " + m.GetContent() + "\n")
	}

	resp, err := d.Model.GenerateContent(ctx, []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextPart("请将对话内容进行高度结构化压缩总结，控制在200字以内。"),
			},
		},
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(summarizeText.String()),
			},
		},
	})
	if err != nil {
		return err
	}

	summary := resp.Choices[0].Content

	// 清空原记忆
	mem.ChatHistory.Clear(ctx)

	// 写入摘要
	mem.ChatHistory.AddMessage(ctx, llms.SystemChatMessage{
		Content: d.SummaryPrefix + summary,
	})

	// 写回保留部分
	for _, m := range toKeep {
		mem.ChatHistory.AddMessage(ctx, m)
	}

	fmt.Printf(">>> [Archiver] 压缩完成，保留最近 %d 轮\n", d.KeepLastRounds)

	return nil
}
func estimateTokens(text string) int {
	// 简单估算：1 token ≈ 4 字符
	return len(text) / 4
}
