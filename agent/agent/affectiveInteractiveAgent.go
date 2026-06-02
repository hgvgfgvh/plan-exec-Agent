package agent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/agent/soul"
	"AgentTest/body/blackboard"
	"AgentTest/capabilities"
	"AgentTest/config"
	_func "AgentTest/func"
	"AgentTest/func/behaveFunc/attention"
	"AgentTest/func/soulChange"
	_speach "AgentTest/func/speach"
	"AgentTest/manager"
	"AgentTest/memory"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"AgentTest/prefrontalCortex"
	"AgentTest/util/sendTopic"
	"context"
	"fmt"
	"strings"

	AffectiveInteractiveAgentRouter "AgentTest/func/Router/AffectiveInteractiveAgent"

	"github.com/tmc/langchaingo/tools"
)

// AffectiveInteractiveAgent 基础智能体：支持长期记忆固化与多工具调度
type AffectiveInteractiveAgent struct {
	Executor     *prefrontalCortex.CustomExecutor
	SystemPrompt string
	ModelKey     string
}

// NewBaseAgent 构造函数
func NewAffectiveInteractiveAgent(soulConfig, modelKey string, ragFilePath string, recallThreshold int) (*AffectiveInteractiveAgent, error) {
	// 1. 动态加载灵魂（人格）
	soulPrompt, err := soul.LoadSoulConfig(soulConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to infuse soul: %v", err)
	}
	prompt := soulPrompt + `

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
- 回答用户时：（表情+上身动作）+自然语言。`
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
	// 2. 实例化反思演化工具
	evolutionTool := &soulChange.ReflectiveEvolutionTool{
		SoulConfigPath: soulConfig, // 注入路径
	}
	// 3. 组装工具列表
	toolList := []tools.Tool{
		_func.MemoryRetrieveTool{
			Searcher: ragProcessor, // 共享 RAG 指针
		},
		archiveTool,
		_speach.CreateSpeechTool(),
		attentionTool,
		evolutionTool,
		AffectiveInteractiveAgentRouter.AffectiveInteractiveAgentOutput{},
	}

	toolList = capabilities.AppendToolsForAgent("interactiveAgent", toolList)

	ex := config.Get().Executor
	agentExecutor := prefrontalCortex.NewCustomExecutor(
		"AffectiveInteractiveAgent",
		m.(*prefrontalCortex.Mode),
		toolList,
		ex.AffectiveMaxSteps,
		ex.AffectiveMaxHistory,
		ragProcessor,
		dialogueHistoryArchiverManager.NewDialogueHistoryArchiverManager(
			m.(*prefrontalCortex.Mode), ex.DialogueArchiveTokens, ex.DialogueArchiveRounds),
	)

	archiveTool.ChatMemory = agentExecutor.Memory //TODO 注意特殊方法 需要反向将NewCustomExecutor中的记忆管理注入

	// 5. 预设系统提示词 (注入运行逻辑约束)

	ba := &AffectiveInteractiveAgent{
		Executor:     agentExecutor,
		SystemPrompt: prompt,
		ModelKey:     modelKey,
	}

	// 让工具直接控制 ba 的 SystemPrompt
	attentionTool.TargetPromptPtr = &ba.SystemPrompt //TODO 注意特殊方法 需要反向将系统提示词注入

	return ba, nil
}

func (ba *AffectiveInteractiveAgent) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 动态提取并校验 query
	var query string
	if len(args) > 0 {
		switch v := args[0].(type) {
		case string:
			query = v
		case fmt.Stringer:
			query = v.String()
		default:
			// 兜底处理：尝试转为字符串格式
			query = fmt.Sprintf("%v", v)
		}
	}

	if query == "" {
		return nil, fmt.Errorf("query is empty or invalid")
	}

	// 2. 执行核心逻辑
	// ba.Executor.Run 会处理内部的工具调用循环和记忆检索
	result, err := ba.Executor.Run(ctx, ba.SystemPrompt, query)
	if err != nil {
		return nil, fmt.Errorf("base agent execution error: %w", err)
	}

	// 3. 返回切片格式以适配插件化架构
	// 后续可以根据需要在此切片中加入 Metadata 或其他辅助信息
	return []interface{}{result}, nil
}

func (ba *AffectiveInteractiveAgent) StartListening(ctx context.Context) {
	envCh := blackboard.GetInstance().Subscribe(blackboard.TopicEnvChange, 10) // 假设有其他 Agent 上报环境
	affectiveInteractiveAgentCh := blackboard.GetInstance().Subscribe(blackboard.TopicAffectiveInput, 10)
	dispatchCh := blackboard.GetInstance().Subscribe(blackboard.TopicAffectiveDispatch, 10)

	go func() {
		for {
			select {
			case msg := <-envCh:
				ba.handleFeedback("【环境感知通知】", msg)
			case msg := <-affectiveInteractiveAgentCh:
				ba.handleFeedback("【路由中心传递来的信息】", msg)
			case msg := <-dispatchCh:
				ba.handleRouterDispatch(msg)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// package agent

// reflectionMaxHops 同 Router 端，配置不在时给出与 Router 一致的兜底默认 2。
func reflectionMaxHops() int {
	if cfg := config.TryGet(); cfg != nil && cfg.Executor.RouterReflectionMaxHops > 0 {
		return cfg.Executor.RouterReflectionMaxHops
	}
	return 2
}

func (ba *AffectiveInteractiveAgent) handleRouterDispatch(msg blackboard.Message) {
	query := fmt.Sprintf("%v", msg.Payload)
	if strings.TrimSpace(query) == "" {
		return
	}
	// 用户首次进入分区：把 query 本身记作 UserQuery，供下游反思链作「对齐用户原话」用途。
	jobCtx := runcontrol.WithTurnMeta(runcontrol.CurrentTurn(), msg.TurnID, msg.Hop)
	jobCtx = runcontrol.WithUserQuery(jobCtx, query)
	runcontrol.AffectiveQ.Submit(jobCtx, func(c context.Context) {
		fmt.Printf("\n[🧠 交互门面：丘脑投递主对话]\n")
		result, err := ba.Executor.Run(c, ba.SystemPrompt, query)
		if err != nil {
			sendTopic.PublishFacadeDedup(msg.TurnID, fmt.Sprintf("交互Agent执行出错: %v", err))
			return
		}
		sendTopic.PublishFacadeDedup(msg.TurnID, result)
	})
}

func (ba *AffectiveInteractiveAgent) handleFeedback(prefix string, msg blackboard.Message) {
	// hop 预算：Affective 处在反思链尾端，无限制会导致 Router↔Affective↔Behavior 反复回声
	// （旧版会产出「已收到…」紧跟「已确认…」两条几乎同义的回复给用户）。
	if msg.Hop >= reflectionMaxHops() {
		fmt.Printf("\n[🛑 Affective 反思预算耗尽] turn=%s hop=%d max=%d 丢弃 %s\n",
			msg.TurnID, msg.Hop, reflectionMaxHops(), prefix)
		return
	}

	// 反思模式 prompt 指令：明确禁止再次调用 AffectiveInteractiveAgentOutput，
	// 否则模型可能把"刚生成的用户回复"又作为 data 路回路由中心，引发第二轮反思回声。
	// 同时附上用户原始诉求（若有）做对齐——这是把 Behavior 的执行结果按用户视角再包装的最便宜手段。
	userQuery := runcontrol.UserQueryFromContext(runcontrol.CurrentTurn())
	eventInfo := fmt.Sprintf("%s: %v", prefix, msg.Payload)
	var sb strings.Builder
	sb.WriteString("【反思模式·禁止再次路由】你现在基于路由中心反馈生成最终用户回复。\n")
	sb.WriteString("- 直接输出自然语言（按「回答用户时」的语义/语气格式）；\n")
	sb.WriteString("- 不要再调用 AffectiveInteractiveAgentOutput；它只用于把任务转出去给行为脑区，本场景不是转出。\n")
	if strings.TrimSpace(userQuery) != "" {
		sb.WriteString(fmt.Sprintf("- 用户原始诉求：%s\n", userQuery))
		sb.WriteString("- 请按「用户原始诉求」的视角，把下面这份反馈精炼为对用户有用的回答（去除内部派发措辞）。\n")
	}
	sb.WriteString(fmt.Sprintf("\n【自主思考触发】收到实时反馈如下：%s", eventInfo))
	reflexQuery := sb.String()

	jobCtx := runcontrol.WithTurnMeta(runcontrol.CurrentTurn(), msg.TurnID, msg.Hop)
	jobCtx = runcontrol.WithUserQuery(jobCtx, userQuery)
	runcontrol.AffectiveQ.Submit(jobCtx, func(c context.Context) {
		fmt.Printf("\n[🧠 Agent 正在思考反馈...] %s\n", prefix)
		result, err := ba.Executor.Run(c, ba.SystemPrompt, reflexQuery)
		if err != nil {
			fmt.Printf("Agent 自主思考出错: %v\n", err)
			sendTopic.PublishFacadeDedup(msg.TurnID, fmt.Sprintf("交互门面反思出错: %v", err))
			return
		}
		out := strings.TrimSpace(result)
		if strings.HasPrefix(out, "Thought: I have the answer now.") {
			out = strings.TrimSpace(strings.TrimPrefix(out, "Thought: I have the answer now."))
		}
		if strings.EqualFold(out, "CONTINUE") || out == "" {
			return
		}
		if strings.Contains(result, "Action:") {
			fmt.Printf("\n[🔥 Agent 主动决策/含 Action 片段] 仍向用户投递门面输出\n")
		} else {
			fmt.Printf("\n[🔥 Agent 未产生主动决策]\n")
		}
		sendTopic.PublishFacadeDedup(msg.TurnID, out)
	})
}
func (ba *AffectiveInteractiveAgent) ReportActionResult(skillName string, out []interface{}, err error) {
}
