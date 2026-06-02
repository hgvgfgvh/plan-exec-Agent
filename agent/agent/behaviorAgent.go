package agent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/behavior/skill"
	"AgentTest/body/blackboard"
	"AgentTest/capabilities"
	"AgentTest/config"
	"AgentTest/experience"
	_func "AgentTest/func"
	"AgentTest/func/behaveFunc"
	"AgentTest/func/behaveFunc/attention"
	experienceFunc "AgentTest/func/behaveFunc/experience"
	"AgentTest/manager"
	"AgentTest/memory"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"AgentTest/plan/planstep"
	"AgentTest/plan/soulhook"
	"AgentTest/prefrontalCortex"
	"AgentTest/util/sendTopic"
	"context"
	"fmt"
	"strings"

	BehaviorAgentAgentRouter "AgentTest/func/Router/BehaviorAgentAgent"

	"github.com/tmc/langchaingo/tools"
)

// BehaviorAgent 行为编排助手对象
type BehaviorAgent struct {
	Executor       *prefrontalCortex.CustomExecutor
	SystemPrompt   string
	ConfigPath     string
	ZeroState      bool
	ExperiencePath string
}

// NewBehaviorAgent 构造函数：初始化模型、工具和技能配置
func NewBehaviorAgent(configPath string, modelKey string, ragFilePath string, recallThreshold int, zeroState bool, experiencePath string) (*BehaviorAgent, error) {
	// 5. 预设系统提示词
	workDir := config.Get().ResolvedPaths().Workspace
	prompt := fmt.Sprintf(`你是行为编排助手：根据用户需求选择并调用工具完成任务。

【工具调用·统一 ReAct】
Action: <工具名>
Action Input: <合法 JSON 对象，勿手写转义外的引号>

【约束】
- 禁止编造工具返回；须真实调用后再总结。
- 问能力/MCP/SKILL：仅复述 system 文末 AGENTS.md，禁止编造未列出的技能或 Domain 内容。
- 查 SQLite 表/跑 SQL/读写文件/发邮件：必须 Action: 调用 AGENTS.md 中的 MCP 公开名（如 sqlite__list_tables），勿用 get_capability_details 代替执行。
- 工具调用格式只能是 Action: 与 Action Input: 两行；禁止 DSML/XML 伪调用。
- 工作区：%s。

【Plan 单步 · report_step_result】
收到【PlanAgent 单步执行】或必须调用 report_step_result 时：
- summary 必须是用户最终应看到的完整正文（问候语、列表、结论、错误说明等），不是步骤执行汇报。
- 禁止在 summary 中只写「完成本步」「已向用户回复」「获取完毕」「下面将展示」等元描述；寒暄/致谢须把真实问候原文写入 summary。
- 若你在调用工具前的回复里已写好给用户的内容，须将同一段原文原样填入 summary，勿改写成第三人称汇报句。

【自主思考】收到【自主思考触发】：AbortExecutorStep / SetExecutorStep / CONTINUE / Output。
`, workDir) + "\n\n" + soulhook.ReferenceOnlyNotice
	// 1. 加载技能配置
	if err := skill.GlobalManager.LoadConfig(configPath); err != nil {
		return nil, fmt.Errorf("load skill config failed: %w", err)
	}

	ragProcessor := memory.NewMyRAGProcessor(recallThreshold, ragFilePath)
	archiveTool := &_func.ArchiveMemoryTool{
		Archiver: ragProcessor, // 已经有了
	}
	experienceManager, error := experience.NewSqliteExperienceManager(experiencePath)
	if error != nil {
		return nil, error
	}

	experienceRetrieveTool := &experienceFunc.ExperienceRetrieveTool{
		Searcher: experienceManager,
	}

	experienceStoreTool := &experienceFunc.ExperienceStoreTool{
		Storer: experienceManager,
	}

	attentionTool := &attention.UpdateWorkingMemoryTool{
		BasePrompt: prompt,
	}

	// 2. 获取模型实例
	m, ok := manager.ModelManager.ModelMap[modelKey]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelKey)
	}

	// 3. 注册工具
	toolList := []tools.Tool{
		behaveFunc.CreateSetExecutorStep(),
		behaveFunc.CreateReportStepResult(),
		archiveTool,
		behaveFunc.CreateAbortExecutorStep(),
		experienceRetrieveTool,
		experienceStoreTool,
		attentionTool,
		BehaviorAgentAgentRouter.BehaviorAgentAgentOutput{},
	}

	toolList = capabilities.AppendToolsForAgent("behaviorAgent", toolList)

	ex := config.Get().Executor
	agentExecutor := prefrontalCortex.NewCustomExecutor(
		"BehaviorAgent",
		m.(*prefrontalCortex.Mode),
		toolList,
		ex.BehaviorMaxSteps,
		ex.BehaviorMaxHistory,
		nil,
		dialogueHistoryArchiverManager.NewDialogueHistoryArchiverManager(
			m.(*prefrontalCortex.Mode), ex.DialogueArchiveTokens, ex.BehaviorArchiveRounds),
	)
	archiveTool.ChatMemory = agentExecutor.Memory //TODO 注意特殊方法 需要反向将NewCustomExecutor中的记忆管理注入

	ba := &BehaviorAgent{
		Executor:       agentExecutor,
		SystemPrompt:   prompt,
		ConfigPath:     configPath,
		ZeroState:      zeroState,
		ExperiencePath: experiencePath,
	}
	// 让工具直接控制 ba 的 SystemPrompt
	attentionTool.TargetPromptPtr = &ba.SystemPrompt //TODO 注意特殊方法 需要反向将系统提示词注入

	return ba, nil
}

// Process 处理单词对话请求
func (ba *BehaviorAgent) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 动态提取 query
	var query string
	if len(args) > 0 {
		if s, ok := args[0].(string); ok {
			query = s
		} else {
			// 如果第一个参数不是字符串，可以将其序列化为字符串，或者报错
			query = fmt.Sprintf("%v", args[0])
		}
	}

	if query == "" {
		return nil, fmt.Errorf("query is empty or invalid")
	}

	if ba.ZeroState {
		ba.Executor.Memory.ChatHistory.Clear(ctx)
	}
	// PlanAgent 单步下发：清空行为脑区对话记忆，避免跨任务串话（每步指令已含完整上下文）。
	if runcontrol.IsPlanStepExecution(ctx) {
		_ = ba.Executor.Memory.ChatHistory.Clear(ctx)
		if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
			runcontrol.ClearStepReport(turnID)
		}
	}
	// 2. 调用内部 LLM 执行器进行推理与工具调用
	// 注意：ba.Executor.Run 内部会触发工具调用（如 SetExecutorStep）
	result, err := ba.Executor.Run(ctx, ba.SystemPrompt, query)
	if err != nil {
		return nil, fmt.Errorf("agent execution error: %w", err)
	}
	// PlanAgent 单步：补齐 SetExecutorStep 异步结果（executor 内已等待时此处多为缓存命中）。
	if err == nil && runcontrol.IsPlanStepExecution(ctx) {
		planstep.ReconcileSkillStepAfterRun(ctx, &result)
	}

	// 丘脑/交互门面断开后，由行为侧直接投递用户可见回复（与 BehaviorAgentAgentOutput 二选一，避免重复）。
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	userText := strings.TrimSpace(result)
	if strings.HasPrefix(userText, "Thought: I have the answer now.") {
		userText = strings.TrimSpace(strings.TrimPrefix(userText, "Thought: I have the answer now."))
	}
	if !runcontrol.IsPlanStepExecution(ctx) &&
		!ba.Executor.BehaviorOutputPublishedThisRun() && userText != "" &&
		!strings.EqualFold(userText, "CONTINUE") {
		sendTopic.PublishFacadeDedup(turnID, sendTopic.SanitizeFacadeText(userText))
	}

	// 3. 将结果封装为 []interface{} 以适配接口
	// 这样 Process 方法本身也可以作为一个“高级技能”被挂载到另一棵树上
	return []interface{}{result}, nil
}
func (ba *BehaviorAgent) ReportActionResult(skillName string, out []interface{}, err error) {
}

func truncateBehaviorResult(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// package agent
// TODO 主对话流程不阻塞的情况下 增加对环境 skill执行等 事件的监听 并注入对话过程中
func (ba *BehaviorAgent) StartListening(ctx context.Context) {
	eventCh := blackboard.GetInstance().Subscribe(blackboard.TopicExecStepEvent, 10)
	behaviorAgentCh := blackboard.GetInstance().Subscribe(blackboard.TopicBehaviorInput, 10)
	go func() {
		for {
			select {
			case msg := <-eventCh:
				ba.handleFeedback("【系统skill的执行通知】", msg)
			case msg := <-behaviorAgentCh:
				ba.handleFeedback("【路由中心传递到的行为需求】", msg)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// package agent

func (ba *BehaviorAgent) handleFeedback(prefix string, msg blackboard.Message) {
	// PlanAgent 同步编排期间：不处理异步 skill/黑板反馈，避免串话与多余门面输出。
	if runcontrol.IsPlanOrchestrating() {
		return
	}
	curTurn := runcontrol.CurrentTurnID()
	if curTurn != "" && msg.TurnID != "" && msg.TurnID != curTurn {
		fmt.Printf("\n[🛑 Behavior 丢弃过期回合反馈] msg.turn=%s cur=%s %s\n", msg.TurnID, curTurn, prefix)
		return
	}

	// hop 预算：Behavior 在反思链中也可能被 Router 二次调用（Router 收到 Affective.output 后
	// 会反弹到 TopicBehaviorInput），不设上限会跟 Affective 形成回声。
	if msg.Hop >= reflectionMaxHops() {
		fmt.Printf("\n[🛑 Behavior 反思预算耗尽] turn=%s hop=%d max=%d 丢弃 %s\n",
			msg.TurnID, msg.Hop, reflectionMaxHops(), prefix)
		return
	}

	// 注入用户原话给反思 prompt：让 Behavior 在「自主思考触发」时知道用户最初要什么，
	// 避免就着上一轮 skill 反馈无限延展而偏离用户目标。
	userQuery := runcontrol.UserQueryFromContext(runcontrol.CurrentTurn())
	eventInfo := fmt.Sprintf("%s: %v", prefix, msg.Payload)
	var sb strings.Builder
	if strings.TrimSpace(userQuery) != "" {
		sb.WriteString(fmt.Sprintf("【用户原始诉求】%s\n", userQuery))
		sb.WriteString("请把下方反馈解读到「用户原始诉求」的语境下：能直接给用户交付时调用 BehaviorAgentAgentOutput 给出对齐用户期待格式的精炼总结；若还需要继续执行 skill 才调用 SetExecutorStep。\n\n")
	}
	sb.WriteString(fmt.Sprintf("【自主思考触发】收到实时反馈如下：%s", eventInfo))
	reflexQuery := sb.String()

	// 把入口消息的 (TurnID, Hop, UserQuery) 沿调用链注入 ctx；下游工具（BehaviorAgentAgentOutput）
	// 通过 runcontrol.TurnMetaFromContext 读取后，发布到 .output 时会带上相同的 Hop，
	// 供 Router 端做反思跳数预算判断。
	jobCtx := runcontrol.WithTurnMeta(runcontrol.CurrentTurn(), msg.TurnID, msg.Hop)
	jobCtx = runcontrol.WithUserQuery(jobCtx, userQuery)
	bb := blackboard.GetInstance()
	runcontrol.BehaviorQ.Submit(jobCtx, func(c context.Context) {
		fmt.Printf("\n[🧠 Agent 正在思考反馈...] %s\n", prefix)
		result, err := ba.Executor.Run(c, ba.SystemPrompt, reflexQuery)
		if err != nil {
			fmt.Printf("Agent 自主思考出错: %v\n", err)
			errText := fmt.Sprintf("行为编排反思执行失败: %v", err)
			bb.PublishMsg(blackboard.TopicBehaviorOutput, errText, msg.TurnID, msg.Hop)
			sendTopic.PublishFacadeDedup(msg.TurnID, sendTopic.SanitizeFacadeText(errText))
			return
		}
		alreadyViaOutput := ba.Executor.BehaviorOutputPublishedThisRun()
		if strings.Contains(result, "Action:") {
			fmt.Printf("\n[🔥 Agent 主动决策] > %s\nUser > ", result)
		} else if alreadyViaOutput {
			fmt.Printf("\n[🔥 Agent 已通过 BehaviorAgentAgentOutput 投递路由]\n")
		} else {
			fmt.Printf("\n[🔥 Agent 未产生 Action 行；尝试将自然语言结论投递至路由]\n")
		}
		userText := strings.TrimSpace(result)
		if strings.HasPrefix(userText, "Thought: I have the answer now.") {
			userText = strings.TrimSpace(strings.TrimPrefix(userText, "Thought: I have the answer now."))
		}
		if alreadyViaOutput {
			return
		}
		if strings.EqualFold(strings.TrimSpace(userText), "CONTINUE") || userText == "" {
			placeholder := "行为编排本轮已结束（模型返回空或 CONTINUE）。若使用了 SetExecutorStep，技能在后台异步执行，请查看日志/黑板 exec 状态。"
			bb.PublishMsg(blackboard.TopicBehaviorOutput, placeholder, msg.TurnID, msg.Hop)
			sendTopic.PublishFacadeDedup(msg.TurnID, sendTopic.SanitizeFacadeText(placeholder))
			return
		}
		bb.PublishMsg(blackboard.TopicBehaviorOutput, userText, msg.TurnID, msg.Hop)
		sendTopic.PublishFacadeDedup(msg.TurnID, sendTopic.SanitizeFacadeText(userText))
		fmt.Printf("\n[🧠 Behavior 反思] 已将结论投递至 agent.BehaviorAgent.output（无 Action 行）\n")
	})
}
