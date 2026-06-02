package agent

import (
	"AgentTest/agent/dispatcher"
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"AgentTest/config"
	_func "AgentTest/func"
	AffectiveInteractiveAgentRouter "AgentTest/func/Router/AffectiveInteractiveAgent"
	BehaviorAgentAgentRouter "AgentTest/func/Router/BehaviorAgentAgent"
	"AgentTest/func/behaveFunc/attention"
	"AgentTest/manager"
	"AgentTest/memory"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"AgentTest/prefrontalCortex"
	"AgentTest/util/sendTopic"
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// RouterAgent 路由智能体分发协调不同的消息进入不同的agent中
type RouterAgent struct {
	Executor     *prefrontalCortex.CustomExecutor
	SystemPrompt string
	ModelKey     string
}

// NewBaseAgent 构造函数
func NewRouterAgent(modelKey string, ragFilePath string, recallThreshold int) (*RouterAgent, error) {
	prompt := `你是一个数字生命的“丘脑路由”。你负责协调各个脑分区 Agent，将消息合理分发到不同的脑分区中【注意！分发的时候不能删减信息】
【脑分区】
情感直接交互脑分区agent：直接和用户沟通的分区，处理交互
行为编排调度脑分区agent：具体行为指令执行的分区。处理行为的执行

【路由身份锁定：你不是用户对话界面，你是“丘脑”。严禁直接对用户说话。你的所有回复必须通过 Action 调用工具完成】

【能力边界】本层不挂载 MCP/外挂 SKILL/能力目录。查库、文件系统、联网搜索等可执行需求一律通过 BehaviorAgentAgentInput 交给行为脑区（该层 system 含 Agent 能力目录）；纯对话与知识问答走 AffectiveInteractiveAgentInput。

【主用户输入路由约定】
- 闲聊、情感、知识问答、解释说明、且不涉及本机执行/文件/邮件/自动化：调用 AffectiveInteractiveAgentInput，JSON 必须包含完整用户语义 {"data":"..."}。
- 打开软件、键鼠、PowerShell、生成 Word/PPT、发邮件、联网搜索落盘、屏幕操作等可执行动作：调用 BehaviorAgentAgentInput，{"data":"可执行需求描述"}。
- 若用户一句话同时包含「解释 + 执行」，可同时调用上述两个工具（各传一份完整相关文本）。

【运行逻辑】：
1. 收到问题后，判断是否需要调用工具。
2. 可以按需求 同时调用多个工具
3. 注意：你只负责调用工具，绝对禁止自行编造【工具的返回结果】。
4. 只要收到工具结果（无论是否查到），你必须立即根据该结果给出自然语言总结，【严禁再次重复调用】相同的工具（如果传递进入的参数不同，可以重复调用）。
5. 注意 对无用的对话记忆进行清除

输出格式：
- 需调工具时：
Action: 工具名
Action Input: {"key":"val"}
`
	// 1. 获取模型
	m, ok := manager.ModelManager.ModelMap[modelKey]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelKey)
	}
	attentionTool := &attention.UpdateWorkingMemoryTool{
		BasePrompt: prompt,
	}
	// 2. 初始化 RAG 处理器 (长期记忆与直觉固化)
	// recallThreshold: 同一关键词检索多少次后转为直觉
	ragProcessor := memory.NewMyRAGProcessor(recallThreshold, ragFilePath)

	archiveTool := &_func.ArchiveMemoryTool{
		Archiver: ragProcessor, // 已经有了
	}
	// 3. 组装工具列表
	toolList := []tools.Tool{
		_func.MemoryRetrieveTool{
			Searcher: ragProcessor, // 共享 RAG 指针
		},
		archiveTool,
		attentionTool,
		AffectiveInteractiveAgentRouter.AffectiveInteractiveAgentInput{},
		BehaviorAgentAgentRouter.BehaviorAgentAgentInput{},
	}

	ex := config.Get().Executor
	agentExecutor := prefrontalCortex.NewCustomExecutor(
		"RouterAgent",
		m.(*prefrontalCortex.Mode),
		toolList,
		ex.RouterMaxSteps,
		ex.RouterMaxHistory,
		ragProcessor,
		dialogueHistoryArchiverManager.NewDialogueHistoryArchiverManager(
			m.(*prefrontalCortex.Mode), ex.DialogueArchiveTokens, ex.DialogueArchiveRounds),
	)

	archiveTool.ChatMemory = agentExecutor.Memory //TODO 注意特殊方法 需要反向将NewCustomExecutor中的记忆管理注入

	// 5. 预设系统提示词 (注入运行逻辑约束)

	ba := &RouterAgent{
		Executor:     agentExecutor,
		SystemPrompt: prompt,
		ModelKey:     modelKey,
	}

	// 让工具直接控制 ba 的 SystemPrompt
	attentionTool.TargetPromptPtr = &ba.SystemPrompt //TODO 注意特殊方法 需要反向将系统提示词注入

	return ba, nil
}

func (ba *RouterAgent) ReportActionResult(skillName string, out []interface{}, err error) {

}

// Process 处理一次用户输入。
//
// 双路由架构：
//
//  1. 快路径（确定性 Dispatcher）
//     - 命中规则 → 直接通过黑板把 query 投递给目标分区，**不调用 LLM**；
//     - 节省一次推理成本、降低延迟、给路由提供可调试/可重放的语义边界。
//
//  2. 慢路径（LLM 兜底）
//     - 仅当 Dispatcher 判定 LowConfidence 时进入；
//     - 复用原 Executor.Run + 工具调用循环，保留 Prompt 工程能撑住的"长尾灵活性"；
//     - 旧"空输出 retry"兜底依然保留，作为最后一道纠偏。
//
// 任一路径都通过 runcontrol.WithTurnMeta 把 TurnID/Hop 注入 ctx，
// 下游工具发布 .output 时会沿用同一回合 ID，反思预算照常生效。
func (ba *RouterAgent) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	var query string
	if len(args) > 0 {
		switch v := args[0].(type) {
		case string:
			query = v
		case fmt.Stringer:
			query = v.String()
		default:
			query = fmt.Sprintf("%v", v)
		}
	}
	if query == "" {
		return nil, fmt.Errorf("query is empty or invalid")
	}

	// 把用户原话放到 ctx 上：即使 BeginTurn 已注入过，再来一次幂等，
	// 兜底覆盖 RunRouterTurn 之外的调用方（例如内部测试）。
	ctx = runcontrol.WithUserQuery(ctx, query)

	// 1. 快路径：确定性 Dispatcher
	decision := dispatcher.Classify(query)
	if decision.Confidence == dispatcher.HighConfidence {
		summary := ba.fastDispatch(ctx, query, decision)
		return []interface{}{summary}, nil
	}

	// 2. 慢路径：LLM 兜底
	fmt.Printf("\n[🧭 Router 走 LLM 兜底] reason=%s\n", decision.Reason)
	result, err := ba.Executor.Run(ctx, ba.SystemPrompt, query)
	if err != nil {
		return nil, fmt.Errorf("router LLM run: %w", err)
	}
	// 丘脑兜底：若首轮完全无输出，追加纠偏提示再跑一轮（防止模型漏工具）。
	if strings.TrimSpace(result) == "" {
		const hint = "\n\n【系统纠偏】你作为丘脑未输出有效内容。请仅使用 AffectiveInteractiveAgentInput 与/或 BehaviorAgentAgentInput，JSON 必须包含 {\"data\":\"...\"}，完整保留用户语义。"
		result, err = ba.Executor.Run(ctx, ba.SystemPrompt, query+hint)
		if err != nil {
			return nil, fmt.Errorf("router retry: %w", err)
		}
	}
	return []interface{}{result}, nil
}

// fastDispatch 在确定性 Dispatcher 命中时执行；直接把 query 投递到目标分区的
// 入口主题，不经 LLM 推理。返回一段人类可读的小结，供 portal 显示。
//
// 不变量：
//   - 投递的消息携带当前回合的 TurnID 与 Hop=0（首次进入反思链前）；
//   - 目标分区 StartListening 处会按各自语义异步消费（Affective 通过 dispatch 主题
//     走 handleRouterDispatch，Behavior 通过 input 主题走 handleFeedback）。
func (ba *RouterAgent) fastDispatch(ctx context.Context, query string, d dispatcher.Decision) string {
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	if turnID == "" {
		turnID = runcontrol.CurrentTurnID()
	}
	bb := blackboard.GetInstance()

	var routed []string
	for _, t := range d.Targets {
		switch t {
		case dispatcher.TargetAffective:
			bb.PublishMsg(blackboard.TopicAffectiveDispatch, query, turnID, 0)
			routed = append(routed, "Affective")
		case dispatcher.TargetBehavior:
			bb.PublishMsg(blackboard.TopicBehaviorInput, query, turnID, 0)
			routed = append(routed, "Behavior")
		}
	}
	summary := fmt.Sprintf("[Dispatcher] %s → %s（turn=%s）",
		d.Reason, strings.Join(routed, "+"), turnID)
	fmt.Printf("\n[🧭 Router 快路径] %s\n", summary)
	return summary
}

func (ba *RouterAgent) StartListening(ctx context.Context) {
	affectiveInteractiveAgentCh := blackboard.GetInstance().Subscribe(blackboard.TopicAffectiveOutput, 10) // 情感直接交互脑分区agent输入
	behaviorAgentCh := blackboard.GetInstance().Subscribe(blackboard.TopicBehaviorOutput, 10)              // 行为编排调度脑分区agent输入

	go func() {
		for {
			select {
			case msg := <-affectiveInteractiveAgentCh:
				ba.handleFeedback("【情感直接交互脑分区agent输入】", msg)
			case msg := <-behaviorAgentCh:
				ba.handleFeedback("【行为编排调度脑分区agent输入】", msg)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// handleFeedback 处理来自其它脑分区的输出消息，并对反思转发施加跳数预算。
//
// 设计要点：
//   - 入参 msg 携带 TurnID / Hop 元信息；Hop 表示本消息已经历的 Agent→Agent 跳数。
//   - 若 msg.Hop 已达到 RouterReflectionMaxHops，则**丢弃**本次反思转发与本次 Executor.Run，
//     防止 Affective ↔ Router ↔ Behavior 通过工具调用形成无界回路。
//   - 仍在预算内时：
//   - 反思转发到对侧 Agent 的 input 时，Hop = msg.Hop + 1；
//   - 当前 Router.Executor.Run 的 ctx 也注入 (TurnID, msg.Hop+1)，
//     这样它内部任何 Action 工具发布的下一跳消息都会继承正确的 Hop。
func (ba *RouterAgent) handleFeedback(prefix string, msg blackboard.Message) {
	turnID := msg.TurnID
	if turnID == "" {
		turnID = runcontrol.CurrentTurnID()
	}
	maxHops := 2
	if cfg := config.TryGet(); cfg != nil && cfg.Executor.RouterReflectionMaxHops > 0 {
		maxHops = cfg.Executor.RouterReflectionMaxHops
	}
	if msg.Hop >= maxHops {
		fmt.Printf("\n[🛑 Router 反思预算耗尽] turn=%s hop=%d max=%d 丢弃 %s\n",
			turnID, msg.Hop, maxHops, prefix)
		return
	}
	nextHop := msg.Hop + 1

	eventInfo := fmt.Sprintf("%s: %v", prefix, msg.Payload)
	reflexQuery := fmt.Sprintf(
		"【脑分区agent消息上报】收到实时反馈如下：%s",
		eventInfo,
	)
	bb := blackboard.GetInstance()
	if prefix == "【情感直接交互脑分区agent输入】" {
		bb.PublishMsg(blackboard.TopicBehaviorInput, reflexQuery, turnID, nextHop)
	}
	if prefix == "【行为编排调度脑分区agent输入】" {
		bb.PublishMsg(blackboard.TopicAffectiveInput, reflexQuery, turnID, nextHop)
	}

	jobCtx := runcontrol.WithTurnMeta(runcontrol.CurrentTurn(), turnID, nextHop)
	runcontrol.RouterQ.Submit(jobCtx, func(c context.Context) {
		fmt.Printf("\n[🧠 Router 丘脑反思] %s turn=%s hop=%d\n", prefix, turnID, nextHop)
		result, err := ba.Executor.Run(c, ba.SystemPrompt, reflexQuery)
		if err != nil {
			fmt.Printf("Router 自主思考出错: %v\n", err)
			sendTopic.PublishFacadeDedup(turnID, fmt.Sprintf("路由反思出错: %v", err))
			return
		}
		if strings.Contains(result, "Action:") {
			fmt.Printf("\n[🔥 Router 主动决策] > %s\n", result)
		}
	})
}
