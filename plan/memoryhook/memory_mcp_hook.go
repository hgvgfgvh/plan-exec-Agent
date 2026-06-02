package memoryhook

import (
	"AgentTest/config"
	"AgentTest/plan/todolist"
	"context"
	"fmt"
	"strings"
)

// RouteDecision Plan 在 Exec 与 Exec-Simple 之间的路由结果。
type RouteDecision struct {
	UseSimple  bool
	Experience Experience
	SkipReason string
}

// RouteInput 路由判定输入（由 PlanAgent 在拆步后传入，不含路由规则本身）。
type RouteInput struct {
	Document            *todolist.Document
	SimpleExecutorReady bool
}

// MemoryMCPHook 将 Memory MCP（或插件 Provider）与 Exec-Simple 路由解耦；PlanAgent 仅调用 DecideRoute。
type MemoryMCPHook struct {
	cfg      *config.App
	provider Provider
}

// NewMemoryMCPHook 根据配置构造钩子；provider 为空时使用 noop。
func NewMemoryMCPHook(cfg *config.App, provider Provider) (*MemoryMCPHook, error) {
	if cfg == nil {
		return nil, fmt.Errorf("memoryhook: nil config")
	}
	if provider == nil {
		var err error
		provider, err = buildProvider(cfg)
		if err != nil {
			return nil, err
		}
	}
	return &MemoryMCPHook{cfg: cfg, provider: provider}, nil
}

// ProviderName 当前挂载的 Provider 插件名。
func (h *MemoryMCPHook) ProviderName() string {
	if h == nil || h.provider == nil {
		return ""
	}
	return h.provider.Name()
}

// DecideRoute 判定是否走 Exec-Simple；护栏与阈值集中在本包，PlanAgent 不嵌入细节。
func (h *MemoryMCPHook) DecideRoute(ctx context.Context, in RouteInput) RouteDecision {
	var zero RouteDecision
	if h == nil || h.cfg == nil || in.Document == nil {
		return zero
	}
	e := h.cfg.Executor
	if !h.cfg.PlanMemoryHook.Enabled || !e.ExecSimpleEnabled || !in.SimpleExecutorReady {
		return RouteDecision{SkipReason: "memory_hook_or_exec_simple_disabled"}
	}
	maxTier := maxDocumentTier(in.Document)
	if maxTier > e.ExecSimpleMaxTier {
		RecordRouteFeedback(in.Document, fmt.Sprintf("Exec-Simple 路由跳过：最高 tier=%d 超过阈值 %d", maxTier, e.ExecSimpleMaxTier))
		return RouteDecision{SkipReason: "tier_too_high"}
	}
	exp, err := h.provider.Retrieve(ctx, RetrieveRequest{
		UserRequirement: in.Document.UserRequirement,
		Document:        in.Document,
	})
	if err != nil {
		RecordRouteFeedback(in.Document, "Exec-Simple 经验检索失败，回退逐步 Exec: "+err.Error())
		return RouteDecision{SkipReason: "retrieve_error"}
	}
	if !exp.Matched || exp.Confidence < e.ExecSimpleMinConfidence {
		RecordRouteFeedback(in.Document, fmt.Sprintf(
			"Exec-Simple 路由跳过：matched=%v confidence=%.2f threshold=%.2f",
			exp.Matched, exp.Confidence, e.ExecSimpleMinConfidence,
		))
		return RouteDecision{SkipReason: "no_match_or_low_confidence", Experience: exp}
	}
	return RouteDecision{UseSimple: true, Experience: exp}
}

// RecordRouteFeedback 将路由说明写入 TodoList（可选，供排查）。
func RecordRouteFeedback(doc *todolist.Document, summary string) {
	if doc == nil || strings.TrimSpace(summary) == "" || len(doc.Steps) == 0 {
		return
	}
	doc.AppendFeedback(0, "validate", summary)
	_ = todolist.Save(doc)
}

func maxDocumentTier(doc *todolist.Document) int {
	maxTier := 1
	if doc == nil {
		return maxTier
	}
	for _, s := range doc.Steps {
		if s.Tier > maxTier {
			maxTier = s.Tier
		}
	}
	return maxTier
}
