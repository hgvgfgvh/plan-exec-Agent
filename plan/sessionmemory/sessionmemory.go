// Package sessionmemory 为 PlanAgent 提供跨用户轮次的对话记忆：
// 近期轮次保留在 ConversationBuffer，超出 plan_max_history 写入长效 JSONL，
// token 超限时由 DialogueHistoryArchiver 压缩为摘要，并支持经验库检索。
package sessionmemory

import (
	"AgentTest/config"
	"AgentTest/experience"
	agentmem "AgentTest/memory"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
	lcmemory "github.com/tmc/langchaingo/memory"
)

// Manager PlanAgent 专用会话记忆（与 Behavior CustomExecutor 记忆并行，不共用 ChatHistory）。
type Manager struct {
	Buffer           *lcmemory.ConversationBuffer
	LongTerm         agentmem.LongTermMemoryProvider
	Archiver         *dialogueHistoryArchiverManager.DialogueHistoryArchiverManager
	Experience       experience.ExperienceProvider
	MaxHistoryRounds int
}

// NewFromConfig 按 config/app.yaml 构造 Plan 会话记忆。
func NewFromConfig(cfg *config.App, model llms.Model) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sessionmemory: nil config")
	}
	e := cfg.ResolvedPaths()
	var longTerm agentmem.LongTermMemoryProvider
	if cfg.Executor.PlanJSONLRAGEnabled {
		ragPath := e.PlanMemory
		if ragPath == "" {
			ragPath = e.Memory
		}
		longTerm = agentmem.NewMyRAGProcessor(cfg.Agents.RAGRecallThreshold, ragPath)
	} else {
		fmt.Fprintln(os.Stderr, "[plan/sessionmemory] plan JSONL RAG 已关闭（executor.plan_jsonl_rag_enabled=false）；保留近期对话 buffer")
	}
	expMgr, err := experience.NewSqliteExperienceManager(e.Experience)
	if err != nil {
		return nil, err
	}
	ex := cfg.Executor
	archiveRounds := ex.PlanArchiveRounds
	if archiveRounds <= 0 {
		archiveRounds = ex.DialogueArchiveRounds
	}
	maxHist := ex.PlanMaxHistory
	if maxHist <= 0 {
		maxHist = 8
	}
	return &Manager{
		Buffer:           lcmemory.NewConversationBuffer(),
		LongTerm:         longTerm,
		Archiver:         dialogueHistoryArchiverManager.NewDialogueHistoryArchiverManager(model, ex.DialogueArchiveTokens, archiveRounds),
		Experience:       expMgr,
		MaxHistoryRounds: maxHist,
	}, nil
}

// PrepareUserContext 在调用 Plan LLM 前组装注入 user 消息的上下文块（不含本轮诉求正文）。
func (m *Manager) PrepareUserContext(ctx context.Context, query string) string {
	if m == nil {
		return ""
	}
	_ = m.rollOverflowToLongTerm(ctx)
	if m.Archiver != nil {
		_ = m.Archiver.MaybeArchive(ctx, m.Buffer)
	}
	var b strings.Builder
	if m.LongTerm != nil {
		if instinct, ok := m.LongTerm.GetInstinct(query); ok {
			b.WriteString("【本能联想】\n")
			b.WriteString(instinct)
			b.WriteString("\n\n")
		}
		if msgs, err := m.LongTerm.Retrieve(ctx, query); err == nil && len(msgs) > 0 {
			b.WriteString("【相关长效记忆】\n")
			for _, msg := range msgs {
				role := msg.GetType()
				if role == llms.ChatMessageTypeAI {
					role = "assistant"
				}
				b.WriteString(fmt.Sprintf("[%s] %s\n", role, msg.GetContent()))
			}
			b.WriteString("\n")
		}
	}
	if m.Experience != nil {
		if exp, err := m.Experience.RetrieveExperience(ctx, query); err == nil && strings.TrimSpace(exp) != "" {
			b.WriteString("【相关技能经验（仅供参考）】\n")
			b.WriteString(exp)
			b.WriteString("\n\n")
		}
	}
	if block := m.formatRecentDialogue(ctx); block != "" {
		b.WriteString("【近期对话（含压缩摘要）】\n")
		b.WriteString(block)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// RecordTurn 在本轮 Plan 编排结束后写入一轮 Human/AI 记忆。
func (m *Manager) RecordTurn(ctx context.Context, userQuery, assistantReply string) {
	if m == nil {
		return
	}
	userQuery = strings.TrimSpace(userQuery)
	assistantReply = strings.TrimSpace(assistantReply)
	if userQuery == "" {
		return
	}
	_ = m.Buffer.ChatHistory.AddMessage(ctx, llms.HumanChatMessage{Content: userQuery})
	if assistantReply != "" {
		_ = m.Buffer.ChatHistory.AddMessage(ctx, llms.AIChatMessage{Content: assistantReply})
	}
	_ = m.rollOverflowToLongTerm(ctx)
}

// BuildMessages 构造发给 Plan LLM 的 messages（system + 可选上下文 human + 任务 human）。
func (m *Manager) BuildMessages(systemPrompt, taskUser string, ctxBlock string) []llms.MessageContent {
	msgs := []llms.MessageContent{
		{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextPart(systemPrompt)}},
	}
	if strings.TrimSpace(ctxBlock) != "" {
		msgs = append(msgs, llms.MessageContent{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart(ctxBlock)},
		})
	}
	msgs = append(msgs, llms.MessageContent{
		Role:  llms.ChatMessageTypeHuman,
		Parts: []llms.ContentPart{llms.TextPart(taskUser)},
	})
	return msgs
}

func (m *Manager) rollOverflowToLongTerm(ctx context.Context) error {
	if m == nil || m.LongTerm == nil || m.MaxHistoryRounds <= 0 {
		return nil
	}
	all, err := m.Buffer.ChatHistory.Messages(ctx)
	if err != nil {
		return err
	}
	maxMsgs := m.MaxHistoryRounds * 2
	if len(all) <= maxMsgs {
		return nil
	}
	offset := len(all) - maxMsgs
	expired := all[:offset]
	if err := m.LongTerm.StoreMessages(ctx, expired); err != nil {
		return err
	}
	remaining := make([]llms.ChatMessage, len(all)-offset)
	copy(remaining, all[offset:])
	return m.Buffer.ChatHistory.SetMessages(ctx, remaining)
}

func (m *Manager) formatRecentDialogue(ctx context.Context) string {
	all, err := m.Buffer.ChatHistory.Messages(ctx)
	if err != nil || len(all) == 0 {
		return ""
	}
	var b strings.Builder
	for _, msg := range all {
		switch msg.GetType() {
		case llms.ChatMessageTypeHuman:
			b.WriteString("用户: ")
		case llms.ChatMessageTypeAI:
			b.WriteString("助手: ")
		case llms.ChatMessageTypeSystem:
			b.WriteString("系统: ")
		default:
			b.WriteString("消息: ")
		}
		b.WriteString(msg.GetContent())
		b.WriteString("\n")
	}
	return b.String()
}
