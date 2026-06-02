// Package runcontrol 提供「用户回合」上下文与各 Agent 上的串行 Run 队列，
// 避免黑板事件与主路径 Process 并发触发多个 Executor.Run 交错破坏状态。
package runcontrol

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TurnManager 绑定「当前一行用户输入」对应的可取消 context；新一行会取消上一回合。
type TurnManager struct {
	mu        sync.Mutex
	cur       context.Context
	cancel    context.CancelFunc
	id        string
	userQuery string
}

var turnCounter atomic.Uint64
var planOrchestrating atomic.Bool

func newTurnID() string {
	return fmt.Sprintf("t-%d-%d", time.Now().Unix(), turnCounter.Add(1))
}

func (tm *TurnManager) Begin(parent context.Context, userQuery string) context.Context {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.cancel != nil {
		tm.cancel()
	}
	if tm.id != "" {
		ClearSynthesizeStream(tm.id)
	}
	tm.id = newTurnID()
	tm.userQuery = userQuery
	tm.cur, tm.cancel = context.WithCancel(parent)
	// 把 TurnID + UserQuery 同时挂在 context 上，方便沿调用链直接取（Hop 在入口默认 0）。
	// UserQuery 也被存到 TurnManager 字段上，便于通过 CurrentTurn() 在其它 goroutine 里读到。
	tm.cur = WithTurnMeta(tm.cur, tm.id, 0)
	tm.cur = WithUserQuery(tm.cur, userQuery)
	return tm.cur
}

func (tm *TurnManager) Current() context.Context {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.cur == nil {
		return context.Background()
	}
	return tm.cur
}

func (tm *TurnManager) CurrentID() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.id
}

// CurrentUserQuery 返回当前回合的用户原话（无回合或空回合时为空串）。
func (tm *TurnManager) CurrentUserQuery() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.userQuery
}

var defaultTurn TurnManager

// BeginTurn 开始新的用户输入回合，并取消上一回合的 context。
// userQuery 为本回合的用户原始输入，会被存到 TurnManager 字段并挂到 context 上，
// 后续异步 goroutine 通过 runcontrol.UserQueryFromContext / CurrentUserQuery 都能读到，
// 用于「对齐用户原话」的反思生成场景。
func BeginTurn(parent context.Context, userQuery string) context.Context {
	return defaultTurn.Begin(parent, userQuery)
}

// BeginPlanOrchestration 标记 PlanAgent 正在同步编排（抑制 Behavior 异步反馈与用户门面泄漏）。
func BeginPlanOrchestration() {
	planOrchestrating.Store(true)
}

// EndPlanOrchestration 结束 PlanAgent 编排标记。
func EndPlanOrchestration() {
	planOrchestrating.Store(false)
}

// IsPlanOrchestrating 是否处于 PlanAgent 编排周期内。
func IsPlanOrchestrating() bool {
	return planOrchestrating.Load()
}

// CurrentTurn 返回当前回合 context（无回合时为 Background）。
func CurrentTurn() context.Context {
	return defaultTurn.Current()
}

// CurrentTurnID 返回当前回合 ID（无回合时为空串）。
func CurrentTurnID() string {
	return defaultTurn.CurrentID()
}

// CurrentUserQuery 返回当前回合的用户原话（无回合时为空串）。
// 异步 handleFeedback 在另一 goroutine 里跑、ctx 上未带 UserQuery 时可用这个兜底。
func CurrentUserQuery() string {
	return defaultTurn.CurrentUserQuery()
}

// --- context 元数据：TurnID / Hop / UserQuery ---

type turnMetaKey struct{}
type userQueryKey struct{}
type planStepKey struct{}
type interactionMetaKey struct{}

// InteractionMeta 交互路由元数据（由 interaction.Router 注入，供 portal / journal / MCP 读取）。
type InteractionMeta struct {
	Channel       string
	DeviceID      string
	SessionID     string
	ReplyChannel  string
	ReplyDeviceID string
}

type turnMeta struct {
	TurnID string
	Hop    int
}

// WithTurnMeta 在 ctx 上挂载回合元信息，供 Agent / 工具沿调用链读取。
// 元信息缺省（turnID==""）时会用 CurrentTurnID 兜底，避免下游拿到空串。
func WithTurnMeta(ctx context.Context, turnID string, hop int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if turnID == "" {
		turnID = defaultTurn.CurrentID()
	}
	return context.WithValue(ctx, turnMetaKey{}, turnMeta{TurnID: turnID, Hop: hop})
}

// TurnMetaFromContext 读取 ctx 上的回合元信息；缺省值为 ("", 0)。
// 若 ctx 上无元信息但当前存在回合，则回退到 (CurrentTurnID(), 0)。
func TurnMetaFromContext(ctx context.Context) (turnID string, hop int) {
	if ctx == nil {
		return defaultTurn.CurrentID(), 0
	}
	if v, ok := ctx.Value(turnMetaKey{}).(turnMeta); ok {
		return v.TurnID, v.Hop
	}
	return defaultTurn.CurrentID(), 0
}

// WithUserQuery 在 ctx 上挂载当前回合的「用户原始诉求」原文。
// 用于：下游 Agent（Behavior / Affective）做反思生成时，可读取它，
// 以「用户问的是 X，下游执行得到 Y」的视角写出对齐用户的最终回复，
// 而不是只在内部反馈链里就事论事造成「差一小步」感。
func WithUserQuery(ctx context.Context, q string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userQueryKey{}, q)
}

// UserQueryFromContext 读取当前回合的用户原话；不存在时返回空串。
func UserQueryFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userQueryKey{}).(string); ok {
		return v
	}
	return ""
}

// WithPlanStepExecution 标记当前 Behavior 调用来自 PlanAgent 单步下发（抑制逐步泄漏到用户门面）。
func WithPlanStepExecution(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, planStepKey{}, true)
}

// IsPlanStepExecution 是否 PlanAgent 单步执行上下文。
func IsPlanStepExecution(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Value(planStepKey{}).(bool)
	return ok && v
}

type planSimpleKey struct{}

// WithPlanSimpleExecution 标记当前调用来自 PlanAgent 的 Exec-Simple episode。
// 它需要结构化回传与工具埋点，但不受单步 plan_step_max_steps 限制。
func WithPlanSimpleExecution(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, planSimpleKey{}, true)
}

// IsPlanSimpleExecution 是否 Exec-Simple episode 执行上下文。
func IsPlanSimpleExecution(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Value(planSimpleKey{}).(bool)
	return ok && v
}

// IsPlanControlledExecution 表示由 PlanAgent 发起、需要结构化回执与工具埋点的执行。
func IsPlanControlledExecution(ctx context.Context) bool {
	return IsPlanStepExecution(ctx) || IsPlanSimpleExecution(ctx)
}

// WithInteractionMeta 挂载本回合设备来源与回执目标（Agent 主链不处理回执匹配）。
func WithInteractionMeta(ctx context.Context, m InteractionMeta) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, interactionMetaKey{}, m)
}

// InteractionMetaFromContext 读取交互路由元数据。
func InteractionMetaFromContext(ctx context.Context) (InteractionMeta, bool) {
	if ctx == nil {
		return InteractionMeta{}, false
	}
	v, ok := ctx.Value(interactionMetaKey{}).(InteractionMeta)
	return v, ok
}

type queuedJob struct {
	ctx context.Context
	fn  func(context.Context)
}

// SerialQueue 单协程串行执行提交的任务，保证同一分区上 Run 不重叠。
type SerialQueue struct {
	name string
	ch   chan queuedJob
}

func NewSerialQueue(name string, buf int) *SerialQueue {
	if buf < 1 {
		buf = 1
	}
	return &SerialQueue{name: name, ch: make(chan queuedJob, buf)}
}

func (q *SerialQueue) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-q.ch:
			j.fn(j.ctx)
		}
	}
}

// Submit 将任务放入队列；jobCtx 应在入队时固定（通常为入队瞬间的 CurrentTurn）。
func (q *SerialQueue) Submit(jobCtx context.Context, fn func(context.Context)) {
	if fn == nil {
		return
	}
	select {
	case q.ch <- queuedJob{ctx: jobCtx, fn: fn}:
	case <-jobCtx.Done():
	}
}

var (
	muStart    sync.Mutex
	started    bool
	BehaviorQ  *SerialQueue
	AffectiveQ *SerialQueue
	RouterQ    *SerialQueue
)

// Boot 启动三条串行队列 worker，随 appCtx 结束而退出。
func Boot(appCtx context.Context) {
	muStart.Lock()
	defer muStart.Unlock()
	if started {
		return
	}
	started = true
	BehaviorQ = NewSerialQueue("behavior", 32)
	AffectiveQ = NewSerialQueue("affective", 32)
	RouterQ = NewSerialQueue("router", 16)
	go BehaviorQ.worker(appCtx)
	go AffectiveQ.worker(appCtx)
	go RouterQ.worker(appCtx)
}
