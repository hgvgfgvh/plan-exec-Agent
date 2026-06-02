package sessionmemory

import (
	"context"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
)

type stubLongTerm struct {
	stored int
}

func (s *stubLongTerm) StoreMessages(ctx context.Context, messages []llms.ChatMessage) error {
	s.stored += len(messages)
	return nil
}
func (s *stubLongTerm) Retrieve(ctx context.Context, query string) ([]llms.ChatMessage, error) {
	return nil, nil
}
func (s *stubLongTerm) GetInstinct(query string) (string, bool) { return "", false }

type stubExp struct{}

func (stubExp) StoreExperience(ctx context.Context, query, skillTree string) error { return nil }
func (stubExp) RetrieveExperience(ctx context.Context, query string) (string, error) {
	return "", nil
}

func TestRecordTurnRollsToLongTerm(t *testing.T) {
	lt := &stubLongTerm{}
	m := &Manager{
		Buffer:           memory.NewConversationBuffer(),
		LongTerm:         lt,
		MaxHistoryRounds: 2,
	}
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		m.RecordTurn(ctx, "q"+string(rune('0'+i)), "a"+string(rune('0'+i)))
	}
	if lt.stored == 0 {
		t.Fatalf("expected overflow messages stored in long-term memory")
	}
	all, _ := m.Buffer.ChatHistory.Messages(ctx)
	if len(all) > 4 {
		t.Fatalf("buffer should keep at most 2 rounds (4 msgs), got %d", len(all))
	}
}

func TestPrepareUserContextIncludesRecent(t *testing.T) {
	m := &Manager{
		Buffer:           memory.NewConversationBuffer(),
		LongTerm:         &stubLongTerm{},
		Experience:       stubExp{},
		MaxHistoryRounds: 8,
	}
	ctx := context.Background()
	m.RecordTurn(ctx, "你有那些外挂SKILL", "三个包：a、b、c")
	block := m.PrepareUserContext(ctx, "他们的详细信息")
	if !strings.Contains(block, "外挂SKILL") {
		t.Fatalf("recent dialogue missing from context block: %q", block)
	}
}

func TestBuildMessagesAddsContextHuman(t *testing.T) {
	m := &Manager{}
	msgs := m.BuildMessages("sys", "task", "ctx-block")
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
}

func TestPrepareUserContextSkipsJSONLRAGWhenLongTermNil(t *testing.T) {
	m := &Manager{
		Buffer:           memory.NewConversationBuffer(),
		LongTerm:         nil,
		Experience:       stubExp{},
		MaxHistoryRounds: 8,
	}
	ctx := context.Background()
	m.RecordTurn(ctx, "上一轮问题", "上一轮回答")
	block := m.PrepareUserContext(ctx, "我是谁")
	if strings.Contains(block, "【本能联想】") || strings.Contains(block, "【相关长效记忆】") {
		t.Fatalf("RAG blocks should be absent when LongTerm nil: %q", block)
	}
	if !strings.Contains(block, "【近期对话") {
		t.Fatalf("recent dialogue should remain: %q", block)
	}
}
