package agent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/behavior/skill"
	"AgentTest/capabilities"
	"AgentTest/config"
	"AgentTest/experience"
	_func "AgentTest/func"
	BehaviorAgentAgentRouter "AgentTest/func/Router/BehaviorAgentAgent"
	"AgentTest/func/behaveFunc"
	"AgentTest/func/behaveFunc/attention"
	experienceFunc "AgentTest/func/behaveFunc/experience"
	"AgentTest/manager"
	"AgentTest/memory"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"AgentTest/plan/soulhook"
	"AgentTest/prefrontalCortex"
	"AgentTest/util/sendTopic"
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// ExecSimpleAgent 执行 PlanAgent 下发的 TodoList-simple episode。
// 它与 BehaviorAgent 拥有同一能力体系，但用独立 Executor 和更长的上下文窗口承载快路径 episode。
type ExecSimpleAgent struct {
	Executor     *prefrontalCortex.CustomExecutor
	SystemPrompt string
}

func NewExecSimpleAgent(configPath string, modelKey string, ragFilePath string, recallThreshold int, experiencePath string) (*ExecSimpleAgent, error) {
	workDir := config.Get().ResolvedPaths().Workspace
	prompt := fmt.Sprintf(`你是 Exec-Simple：PlanAgent 的快路径执行体。

【职责】
- 接收 Plan 下发的 TodoList-simple / episode 指令，在一个连续执行上下文内完成多步任务。
- 能力体系与 BehaviorAgent/Exec 一致：MCP 公开工具、内置技能、外挂 SKILL 均按 AGENTS.md 能力目录与 get_capability_details 使用。
- episode 期间不要逐步向 Plan 回报；仅在整个 episode 成功完成，或遇到路径级无法解决错误时，调用一次 report_step_result。

【工具调用·统一 ReAct】
Action: <工具名>
Action Input: <合法 JSON 对象，勿手写转义外的引号>

【约束】
- 禁止编造工具返回；须真实调用后再总结。
- 若遇到权限、工具不存在、数据不可达、反复失败等无法在本 episode 内解决的问题，调用 report_step_result 且 status=fail，让 Plan 降级保守 Exec。
- 成功时 report_step_result 必须包含 summary、artifacts、tools_called；若无产物 artifacts 可为空数组。
- 工作区：%s。
`, workDir) + "\n\n" + soulhook.ReferenceOnlyNotice

	if err := skill.GlobalManager.LoadConfig(configPath); err != nil {
		return nil, fmt.Errorf("load skill config failed: %w", err)
	}

	ragProcessor := memory.NewMyRAGProcessor(recallThreshold, ragFilePath)
	archiveTool := &_func.ArchiveMemoryTool{Archiver: ragProcessor}
	experienceManager, err := experience.NewSqliteExperienceManager(experiencePath)
	if err != nil {
		return nil, err
	}

	experienceRetrieveTool := &experienceFunc.ExperienceRetrieveTool{Searcher: experienceManager}
	experienceStoreTool := &experienceFunc.ExperienceStoreTool{Storer: experienceManager}
	attentionTool := &attention.UpdateWorkingMemoryTool{BasePrompt: prompt}

	m, ok := manager.ModelManager.ModelMap[modelKey]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelKey)
	}

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
	toolList = capabilities.AppendToolsForAgent("execSimpleAgent", toolList)

	ex := config.Get().Executor
	agentExecutor := prefrontalCortex.NewCustomExecutor(
		"ExecSimpleAgent",
		m.(*prefrontalCortex.Mode),
		toolList,
		ex.ExecSimpleMaxSteps,
		ex.ExecSimpleMaxHistory,
		ragProcessor,
		dialogueHistoryArchiverManager.NewDialogueHistoryArchiverManager(
			m.(*prefrontalCortex.Mode), ex.DialogueArchiveTokens, ex.ExecSimpleArchiveRounds),
	)
	archiveTool.ChatMemory = agentExecutor.Memory

	ea := &ExecSimpleAgent{
		Executor:     agentExecutor,
		SystemPrompt: prompt,
	}
	attentionTool.TargetPromptPtr = &ea.SystemPrompt
	return ea, nil
}

func (ea *ExecSimpleAgent) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	query, err := firstStringArg(args...)
	if err != nil {
		return nil, err
	}
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	if turnID != "" {
		runcontrol.ClearStepReport(turnID)
	}
	ctx = runcontrol.WithPlanSimpleExecution(ctx)
	result, err := ea.Executor.Run(ctx, ea.SystemPrompt, query)
	if err != nil {
		return nil, fmt.Errorf("exec-simple execution error: %w", err)
	}
	userText := strings.TrimSpace(result)
	if strings.HasPrefix(userText, "Thought: I have the answer now.") {
		userText = strings.TrimSpace(strings.TrimPrefix(userText, "Thought: I have the answer now."))
	}
	if !ea.Executor.BehaviorOutputPublishedThisRun() && userText != "" && !strings.EqualFold(userText, "CONTINUE") {
		// Simple episode 由 Plan 统一合流；此处仅在非 Plan 上下文误调用时避免静默。
		if !runcontrol.IsPlanControlledExecution(ctx) {
			sendTopic.PublishFacadeDedup(turnID, sendTopic.SanitizeFacadeText(userText))
		}
	}
	return []interface{}{result}, nil
}

func (ea *ExecSimpleAgent) ReportActionResult(skillName string, out []interface{}, err error) {}

func (ea *ExecSimpleAgent) StartListening(ctx context.Context) {
	// Exec-Simple 只接受 PlanAgent 同步下发的 episode，不订阅黑板，避免绕过 Plan 裁决。
}
