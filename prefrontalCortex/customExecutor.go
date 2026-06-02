package prefrontalCortex

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/capabilities"
	"AgentTest/config"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"AgentTest/plan/skillwait"
	"AgentTest/util/sendTopic"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	Amenory "AgentTest/memory"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/tools"
)

// truncateRunes 按 Unicode 标量截断，避免超长文本撑爆上下文。
func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// truncateToolDocForPrompt 限制每轮 system 里「工具清单」单行长度，避免非 MCP 工具的超长 Description 撑爆上下文。
func truncateToolDocForPrompt(s string, maxRunes int) string {
	return truncateRunes(s, maxRunes)
}

// executorRunLimits 从配置读取单次 Run 的 history / 工具回包截断参数（TryGet 失败时用内置默认）。
func executorRunLimits() (historyToolRounds, obsMaxRunes, lineMaxRunes, aiMaxRunes int) {
	historyToolRounds, obsMaxRunes, lineMaxRunes, aiMaxRunes = 12, 16000, 8000, 24000
	if cfg := config.TryGet(); cfg != nil {
		e := cfg.Executor
		if e.HistoryToolRoundsKeep > 0 {
			historyToolRounds = e.HistoryToolRoundsKeep
		}
		if e.ToolObservationMaxRunes > 0 {
			obsMaxRunes = e.ToolObservationMaxRunes
		}
		if e.ToolResultLineMaxRunes > 0 {
			lineMaxRunes = e.ToolResultLineMaxRunes
		}
		if e.AIRoundMaxRunes > 0 {
			aiMaxRunes = e.AIRoundMaxRunes
		}
	}
	return
}

// trimExecutorHistoryTail 保留 history 前部（system + 可选已知历史 + 当前 query）及最近若干轮「助手 + 工具」消息，丢弃更早的工具轮次。
// prefixLen 为进入工具循环前 history 的长度；keepToolRounds 为保留的工具轮数（每轮含 2 条：AI + Tool）。
func trimExecutorHistoryTail(history []llms.MessageContent, prefixLen, keepToolRounds int) []llms.MessageContent {
	if keepToolRounds < 1 {
		keepToolRounds = 1
	}
	if prefixLen < 0 || prefixLen > len(history) {
		return history
	}
	tail := len(history) - prefixLen
	maxTail := keepToolRounds * 2
	if tail <= maxTail {
		return history
	}
	drop := tail - maxTail
	if drop%2 != 0 {
		drop--
		if drop < 0 {
			return history
		}
	}
	out := make([]llms.MessageContent, 0, prefixLen+maxTail)
	out = append(out, history[:prefixLen]...)
	out = append(out, history[prefixLen+drop:]...)
	return out
}

// ==========================================
// 执行器核心逻辑 (CustomExecutor)
// ==========================================
//
// 并发模型：本结构体不持有自身锁。每个 Agent 拥有独立的 CustomExecutor 实例，
// 同一 Agent 的多次 Run 调用由其所属 runcontrol.*Q（FIFO 单 goroutine 执行器）串行化，
// 因此 Memory / executedActions / archiveCallCount 等 Run 内/跨 Run 共享状态在
// 同一 Executor 内天然单线程访问；不同 Agent 的 Executor 互不共享状态。
// behaviorOutputPublished 用 atomic 因仅它会被其它包并发查询（BehaviorOutputPublishedThisRun）。
type CustomExecutor struct {
	AgentName     string
	Model         *Mode
	Tools         map[string]tools.Tool
	Memory        *memory.ConversationBuffer
	MaxSteps      int
	MaxHistoryLen int
	// 新增：RAG 处理器对象
	LongTermMemory Amenory.LongTermMemoryProvider
	// 新增超出token限制的对话内容ai进行对话汇总
	dialogueHistoryArchiverManager *dialogueHistoryArchiverManager.DialogueHistoryArchiverManager
	// 工具说明书（用于 system prompt 拼接），在构造期一次性生成；MCP 工具按 SuppressExecutorToolPrompt 隐藏。
	toolDocsPrompt string
	// 本回合 Run 内是否已成功执行 BehaviorAgentAgentOutput（1=是）。用于反思路径避免重复投递用户可见文本。
	behaviorOutputPublished uint32
}

func NewCustomExecutor(AgentName string, m *Mode, toolList []tools.Tool, maxSteps int, maxHistory int, ragProvider Amenory.LongTermMemoryProvider, dialogueHistoryArchiverManager *dialogueHistoryArchiverManager.DialogueHistoryArchiverManager) *CustomExecutor {
	toolMap := make(map[string]tools.Tool)
	for _, t := range toolList {
		toolMap[t.Name()] = t
	}
	return &CustomExecutor{
		AgentName:                      AgentName,
		Model:                          m,
		Tools:                          toolMap,
		Memory:                         memory.NewConversationBuffer(),
		MaxSteps:                       maxSteps,
		MaxHistoryLen:                  maxHistory,
		LongTermMemory:                 ragProvider,
		dialogueHistoryArchiverManager: dialogueHistoryArchiverManager,
		toolDocsPrompt:                 buildExecutorToolPrompt(toolMap),
	}
}

// BehaviorOutputPublishedThisRun 表示当前这次 Run 中已成功调用过 BehaviorAgentAgentOutput。
func (e *CustomExecutor) BehaviorOutputPublishedThisRun() bool {
	return atomic.LoadUint32(&e.behaviorOutputPublished) != 0
}

// shouldEmitIntermediate 判断当前 Executor（按 AgentName）是否被允许把工具循环过程中的
// 模型中间叙述推送到 facadeInteraction.output（即用户屏幕）。
// 由 config.Executor.FacadeIntermediateAgents 白名单驱动，默认仅 AffectiveInteractiveAgent。
func (e *CustomExecutor) shouldEmitIntermediate() bool {
	cfg := config.TryGet()
	if cfg == nil {
		// 配置不在时退回到「只允许 Affective」的保守默认，与 applyDefaults 一致。
		return e.AgentName == "AffectiveInteractiveAgent"
	}
	for _, n := range cfg.Executor.FacadeIntermediateAgents {
		if n == e.AgentName {
			return true
		}
	}
	return false
}

//`你是一个智能助手。
//%s
//
//【运行逻辑】：
//1. 收到问题后，判断是否需要调用工具。
//2. 可以按需求 同时调用多个工具
//
//
//输出格式：
//- 需调工具时：
//    Action: 工具名
//    Action Input: {"key":"val"}
//- 回答用户时：直接说自然语言。`

func (e *CustomExecutor) Run(ctx context.Context, systemSay string, query string) (string, error) {
	atomic.StoreUint32(&e.behaviorOutputPublished, 0)
	// 【本能层】：无需模型判断，系统先自动搜一下
	// 这里调用我们之前在 MyRAGProcessor 里写的 GetInstinct
	if e.LongTermMemory != nil {
		if instinct, ok := e.LongTermMemory.GetInstinct(query); ok {
			// 将这个“本能”作为【已知背景】直接塞进 history，不占 Action 步数
			query = fmt.Sprintf("（本能联想：关于'%s'，你记得：%s）\n%s", query, instinct, query)
		}
	}
	// 1. 直接使用构造期缓存好的工具说明书（避免每轮 Run 都重新遍历 Tools + 拼字符串）。
	//    MCP 工具按 SuppressExecutorToolPrompt 隐藏，公开发现类工具仍出现在 toolDocsPrompt 中。
	toolDocsStr := e.toolDocsPrompt

	// 2. 加载并“物理”截断记忆 (解决重复存入 RAG 的问题)
	allMessages, _ := e.Memory.ChatHistory.Messages(ctx)
	maxMsgs := e.MaxHistoryLen * 2

	if len(allMessages) > maxMsgs {
		offset := len(allMessages) - maxMsgs

		// A. 提取需要移出内存的消息
		expiredMsgs := allMessages[:offset]

		// B. 同步到 RAG 处理器
		if e.LongTermMemory != nil {
			fmt.Printf(">>> [系统] 发现超出记忆 %d 条，正在同步至 RAG...\n", offset)
			if err := e.LongTermMemory.StoreMessages(ctx, expiredMsgs); err != nil {
				// 如果 RAG 存储失败，为了数据安全，不执行后续清除操作
				return "", fmt.Errorf("RAG同步失败，停止推理以防止记忆丢失: %v", err)
			}
		}

		// C. 物理清理内存：一次 SetMessages 直接覆盖（替代 Clear + 循环 AddMessage 的 N+1 次调用）
		remainingMsgs := make([]llms.ChatMessage, len(allMessages)-offset)
		copy(remainingMsgs, allMessages[offset:])
		if err := e.Memory.ChatHistory.SetMessages(ctx, remainingMsgs); err != nil {
			return "", fmt.Errorf("记忆滚动失败: %v", err)
		}

		fmt.Printf(">>> [系统] 记忆滚动完成：已从内存擦除旧记录，保留最近 %d 轮内容\n", e.MaxHistoryLen)

		// 更新本次推理使用的上下文
		allMessages = remainingMsgs
	}

	// 3. 将当前有效的内存消息格式化为文本
	var historyText strings.Builder
	for _, msg := range allMessages {
		prefix := "Human"
		if msg.GetType() == llms.ChatMessageTypeAI {
			prefix = "AI"
		}
		historyText.WriteString(fmt.Sprintf("%s: %s\n", prefix, msg.GetContent()))
	}

	var history []llms.MessageContent

	//// 4. 构造 System Prompt
	//systemPrompt := fmt.Sprintf(systemSay, toolDocs.String())
	//
	//history = append(history, llms.MessageContent{
	//	Role:  llms.ChatMessageTypeSystem,
	//	Parts: []llms.ContentPart{llms.TextPart(systemPrompt)},
	//})

	// 4. 构造包含「粗粒度」当前时间的 System Prompt。
	//    按 executor.prompt_time_granularity_seconds 向下舍入（默认 5min），让同窗口内 prompt 完全一致，
	//    既保留时间感知，又为 LLM 响应缓存留出命中空间。设为 1 可恢复秒级实时注入。
	now := time.Now()
	granularity := 300 * time.Second
	if cfg := config.TryGet(); cfg != nil && cfg.Executor.PromptTimeGranularitySeconds > 0 {
		granularity = time.Duration(cfg.Executor.PromptTimeGranularitySeconds) * time.Second
	}
	rounded := now.Truncate(granularity)
	var timeLayout string
	if granularity < time.Minute {
		timeLayout = "2006年01月02日 15:04:05"
	} else {
		timeLayout = "2006年01月02日 15:04"
	}
	currentTime := rounded.Format(timeLayout)
	weekday := rounded.Weekday()

	// system 结构：角色说明 → 统一 Function Calling 工具表 → AGENTS.md 能力目录（第一层）
	enhancedSystemSay := fmt.Sprintf(
		"当前时间：%s (星期%s)\n\n%s",
		currentTime,
		translateWeekday(weekday),
		systemSay,
	)
	systemPrompt := enhancedSystemSay + "\n\n" + toolDocsStr
	if catalog := capabilities.FormatCatalogForExecutor(e.AgentName); catalog != "" {
		systemPrompt += catalog
	}

	history = append(history, llms.MessageContent{
		Role:  llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{llms.TextPart(systemPrompt)},
	})

	// 注入截断后的历史
	if historyText.Len() > 0 {
		history = append(history, llms.MessageContent{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("【已知对话历史】:\n" + historyText.String())},
		})
	}

	history = append(history, llms.MessageContent{
		Role:  llms.ChatMessageTypeHuman,
		Parts: []llms.ContentPart{llms.TextPart(query)},
	})

	historyPrefixLen := len(history)
	historyToolRounds, obsMaxRunes, lineMaxRunes, aiMaxRunes := executorRunLimits()

	maxSteps := e.MaxSteps
	if runcontrol.IsPlanStepExecution(ctx) {
		maxSteps = planStepMaxSteps()
	}

	// 用于防止模型死循环的硬拦截 map
	executedActions := make(map[string]bool)
	isQuerySaved := false                            // 新增：标记位，确保 query 不会被循环重复存储
	isReflex := strings.HasPrefix(query, "【自主思考触发】") // 新增：识别是否为反馈触发
	archiveCallCount := 0

	progressiveTools := capabilities.UseProgressiveToolDisclosure(e.AgentName)
	var revealed *capabilities.RevealedToolSet
	if progressiveTools {
		revealed = capabilities.NewRevealedToolSet()
		fmt.Printf("[executor] 渐进披露 tools[]：初始仅元工具 + AGENTS 地图；MCP 须先 get_capability_details 解锁\n")
	}

	// Exec-Simple episode：记录上一轮实际执行的工具名（用于纯文本结束时的兜底补 report）。
	var lastExecutedTool string

	// --- 5. 推理循环 ---
	for i := 0; i < maxSteps; i++ {
		fmt.Printf("\n>>>> [第 %d 轮迭代] <<<<\n", i+1)
		if e.dialogueHistoryArchiverManager != nil {
			if err := e.dialogueHistoryArchiverManager.MaybeArchive(ctx, e.Memory); err != nil {
				fmt.Printf("dialogueHistoryArchiverManagerErr: %s\n", err.Error())
			}
		}

		apiToolMap := capabilities.FilterToolMapForAPI(e.Tools, revealed, progressiveTools)
		if progressiveTools && i == 0 {
			fmt.Printf("[executor] API tools[] 数量: %d / 注册表 %d\n", len(apiToolMap), len(e.Tools))
		}
		resp, err := e.Model.GenerateForExecutor(ctx, history, apiToolMap)
		if err != nil {
			return "", fmt.Errorf("模型推理超时或失败: %v", err)
		}
		answer := resp.Choices[0].Content
		if len(resp.Choices[0].ToolCalls) > 0 {
			fmt.Printf("[模型输出] API tool_calls: %d 条\n", len(resp.Choices[0].ToolCalls))
		}
		fmt.Printf("[模型输出]:\n%s\n", answer)
		if runcontrol.IsPlanControlledExecution(ctx) {
			if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
				runcontrol.MergeStepUserVisible(turnID, answer)
			}
		}
		// --- 修正后的：实时流式存档逻辑 ---
		if !isReflex {
			// 1. 存入用户的问题（仅在第一轮循环执行）
			if !isQuerySaved {
				e.Memory.ChatHistory.AddMessage(ctx, llms.HumanChatMessage{
					Content: query,
				})
				isQuerySaved = true
			}
			// 2. 存入 AI 刚刚产生的回答
			e.Memory.ChatHistory.AddMessage(ctx, llms.AIChatMessage{
				Content: answer,
			})
		} else {
			//todo 为反馈触发 也存入对话历史中【因为有的时候 skill的反馈不仅带有执行状态 还有 执行的内容（比如网络搜索的结果）】
			if !isQuerySaved {
				e.Memory.ChatHistory.AddMessage(ctx, llms.HumanChatMessage{
					Content: query,
				})
				isQuerySaved = true
			}
			// 2. 存入 AI 刚刚产生的回答
			e.Memory.ChatHistory.AddMessage(ctx, llms.AIChatMessage{
				Content: answer,
			})
		}
		// ----------------------------
		actions, isAction := ActionsFromLLMResponse(resp, answer, apiToolMap)
		if isAction {
			actions = normalizeActionList(actions, apiToolMap)
		}
		if isAction && runcontrol.IsPlanControlledExecution(ctx) {
			actions = filterPlanStepActions(actions)
			if len(actions) > 3 {
				actions = actions[:3]
			}
			if len(actions) == 0 {
				isAction = false
			}
		}

		if isAction {
			// 中间叙述泄漏控制：仅当 AgentName 在 executor.facade_intermediate_agents 白名单内
			// 才把本轮模型原文（含思考铺垫，如"好的我先查询…"）推到用户门面。
			// 默认 Affective 在内、Router/Behavior 不在，避免内部独白泄漏给用户。
			if e.shouldEmitIntermediate() {
				sendTopic.SendTopicFacadeInteraction(e.AgentName, answer)
			}
			var apiToolCalls []llms.ToolCall
			if resp != nil && len(resp.Choices) > 0 {
				apiToolCalls = resp.Choices[0].ToolCalls
			}
			if len(apiToolCalls) > 0 {
				reasoning := ""
				if resp != nil && len(resp.Choices) > 0 {
					reasoning = resp.Choices[0].ReasoningContent
				}
				history = AppendAssistantToolCallsHistory(history, answer, apiToolCalls, reasoning)
			} else {
				answerForHistory := truncateRunes(answer, aiMaxRunes)
				history = append(history, llms.MessageContent{Role: llms.ChatMessageTypeAI, Parts: []llms.ContentPart{llms.TextPart(answerForHistory)}})
			}

			obsRaw, executedToolNames, batchErr := e.executeToolBatch(ctx, actions, executedActions, &archiveCallCount, lineMaxRunes, revealed, progressiveTools)
			if batchErr != nil {
				return "", batchErr
			}
			obsStr := truncateRunes(obsRaw, obsMaxRunes)

			// Plan 发起的执行 + SetExecutorStep：阻塞等待真实技能结果。
			// 单步 Exec 会尽量自动写入 report_step_result；Exec-Simple 只把观测交回模型继续 episode，避免过早结束。
			if runcontrol.IsPlanControlledExecution(ctx) && skillwait.MustWaitAfterToolBatch(executedToolNames, obsStr) {
				if runcontrol.IsPlanSimpleExecution(ctx) {
					fmt.Printf("\n[executor] Exec-Simple episode：等待 SetExecutorStep 技能结果…\n")
				} else {
					fmt.Printf("\n[executor] Plan 单步：等待 SetExecutorStep 技能结果…\n")
				}
				waitCtx, cancel := context.WithTimeout(ctx, skillwait.DefaultTimeout)
				skillOut, waitErr := skillwait.Wait(waitCtx, skillwait.DefaultTimeout)
				cancel()
				if waitErr != nil {
					obsStr = obsStr + "\n\n【内置技能执行未完成】" + waitErr.Error()
				} else if strings.TrimSpace(skillOut) != "" {
					obsStr = "【内置技能执行结果】\n" + skillOut
					if runcontrol.IsPlanStepExecution(ctx) && runcontrol.AutoSubmitAfterSkillResult(ctx, strings.TrimSpace(skillOut), executedToolNames) {
						fmt.Printf("\n[executor] 已自动写入 report_step_result（status=ok）\n")
					}
				}
			}
			if len(executedToolNames) > 0 {
				lastExecutedTool = executedToolNames[len(executedToolNames)-1]
			}

			// Router 仅派发至其它脑区时，真实回复由异步通道送达；勿再进入第二轮让模型「汇总」以免重复调用同一派发工具或产生元话语。
			if e.AgentName == "RouterAgent" && len(executedToolNames) > 0 {
				dispatchOnly := true
				for _, n := range executedToolNames {
					if n != "AffectiveInteractiveAgentInput" && n != "BehaviorAgentAgentInput" {
						dispatchOnly = false
						break
					}
				}
				if dispatchOnly {
					if !isReflex {
						e.Memory.ChatHistory.AddMessage(ctx, llms.SystemChatMessage{
							Content: "【系统观测】：工具调用已完成。反馈如下：\n" + obsStr,
						})
					}
					return "（丘脑：已派发至对应脑区，用户可见回复由交互/编排通道送达。）", nil
				}
			}

			// 丘脑纯派发以外的路径：将截断后的工具观测写入 Memory（含自主思考触发），便于 MaybeArchive 统计 token，并避免 reflex 仅靠 history 叠长文。
			e.Memory.ChatHistory.AddMessage(ctx, llms.SystemChatMessage{
				Content: "【系统观测】：工具调用已完成。反馈如下：\n" + obsStr,
			})
			if len(apiToolCalls) == 0 {
				history = append(history, llms.MessageContent{
					Role:  llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{llms.TextPart(obsStr + "\n请根据以上所有结果进行汇总分析。")},
				})
			} else {
				for i, tc := range apiToolCalls {
					tid := tc.ID
					if tid == "" {
						tid = fmt.Sprintf("call_%d", i)
					}
					body := obsStr
					if i > 0 {
						body = fmt.Sprintf("（同批次第 %d 个工具，观测与首条一致。）\n%s", i+1, obsStr)
					}
					history = AppendToolResultHistory(history, tid, body)
				}
			}
			history = trimExecutorHistoryTail(history, historyPrefixLen, historyToolRounds)
			if runcontrol.IsPlanControlledExecution(ctx) {
				if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
					if rep, ok := runcontrol.PeekStepReport(turnID); ok {
						return planStepReturnText(rep), nil
					}
				}
			}
			continue

		} else {
			fmt.Println(">>> 状态: 判定为最终回答")
			if runcontrol.IsPlanControlledExecution(ctx) {
				if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
					if rep, ok := runcontrol.PeekStepReport(turnID); ok {
						return planStepReturnText(rep), nil
					}
				}
			}

			finalOutput := answer
			reClean := regexp.MustCompile(`(?m)^\s*(Thought|Final Answer|Action|Action Input):.*$`)
			finalOutput = reClean.ReplaceAllString(finalOutput, "")
			finalOutput = strings.ReplaceAll(finalOutput, "<think>", "")
			finalOutput = strings.ReplaceAll(finalOutput, "</think>", "")
			finalOutput = sendTopic.StripControlTokens(finalOutput)
			finalOutput = strings.TrimSpace(finalOutput)

			// Exec-Simple：本轮无工具且上一轮工具不是 report_step_result → 用最终正文兜底 episode 回执。
			if runcontrol.IsPlanSimpleExecution(ctx) && lastExecutedTool != "" && lastExecutedTool != "report_step_result" {
				if turnID, _ := runcontrol.TurnMetaFromContext(ctx); turnID != "" {
					if rep, ok := runcontrol.TryAutoSubmitSimpleEpisodeFinal(ctx, finalOutput); ok {
						return planStepReturnText(rep), nil
					}
				}
			}

			//// 保存到 Memory (此时 Memory 只有最近 N 轮，SaveContext 会添加最新的一轮)
			//e.Memory.SaveContext(ctx, map[string]any{"input": query}, map[string]any{"output": finalOutput})
			return finalOutput, nil
		}
	}

	return "", fmt.Errorf("推理达到最大次数上限")
}

// 提取 Action 的辅助方法保持不变...
func (e *CustomExecutor) extractAction(input string) (name string, params string, ok bool) {
	reThink := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleanInput := reThink.ReplaceAllString(input, "")

	reAction := regexp.MustCompile(`Action:\s*(\w+)`)
	reInput := regexp.MustCompile(`Action Input:\s*(\{.*\})`)

	actionMatch := reAction.FindStringSubmatch(cleanInput)
	inputMatch := reInput.FindStringSubmatch(cleanInput)

	if len(actionMatch) > 1 && len(inputMatch) > 1 {
		return strings.TrimSpace(actionMatch[1]), strings.TrimSpace(inputMatch[1]), true
	}
	return "", "", false
}

func (e *CustomExecutor) extractActions(input string) ([]struct{ Name, Params string }, bool) {
	return extractActionBlocks(input)
}

// 辅助函数：将英文星期转换为中文
func translateWeekday(w time.Weekday) string {
	days := []string{"日", "一", "二", "三", "四", "五", "六"}
	return days[w]
}

func planStepReturnText(rep runcontrol.StepReport) string {
	return runcontrol.StepReportUserText(rep)
}

func toolBatchUsedSetExecutorStep(executedToolNames []string) bool {
	for _, n := range executedToolNames {
		if n == "SetExecutorStep" {
			return true
		}
	}
	return false
}

func planStepMaxSteps() int {
	if cfg := config.TryGet(); cfg != nil && cfg.Executor.PlanStepMaxSteps > 0 {
		return cfg.Executor.PlanStepMaxSteps
	}
	return 8
}

func filterPlanStepActions(actions []struct{ Name, Params string }) []struct{ Name, Params string } {
	if len(actions) == 0 {
		return actions
	}
	out := make([]struct{ Name, Params string }, 0, len(actions))
	for _, a := range actions {
		if a.Name == "update_task_dashboard" {
			continue
		}
		out = append(out, a)
	}
	return out
}
