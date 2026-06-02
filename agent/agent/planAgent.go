package agent

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/capabilities"
	"AgentTest/config"
	"AgentTest/manager"
	"AgentTest/outputbus"
	"AgentTest/plan/delivery"
	"AgentTest/plan/memoryhook"
	"AgentTest/plan/planstep"
	"AgentTest/plan/sessionmemory"
	"AgentTest/plan/soulhook"
	"AgentTest/plan/stepmeta"
	"AgentTest/plan/todolist"
	"AgentTest/plan/verify"
	"AgentTest/prefrontalCortex"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

const (
	planStepFailKeywords = `错误：工具|执行失败|工具不存在|invalid JSON|调用失败|statusCode["']?\s*:\s*401|restricted_api_key|未授权|权限不足|API 密钥.*受限|❌|无法获取（|失败:|工具 \[.*\] 结果: 错误|内置技能执行未完成`
)

// planOrchestrationLimits 来自 config/app.yaml executor.plan_*（加载后已填默认值）。
type planOrchestrationLimits struct {
	promptMaxSteps        int
	maxStepsPerPlan       int
	maxDispatchPerTurn    int
	maxAdjustPerStep      int
	resultSummaryMaxRunes int
	stepDetailMaxRunes    int
}

func getPlanOrchestrationLimits() planOrchestrationLimits {
	lim := planOrchestrationLimits{
		promptMaxSteps:        12,
		maxStepsPerPlan:       24,
		maxDispatchPerTurn:    40,
		maxAdjustPerStep:      3,
		resultSummaryMaxRunes: 2000,
		stepDetailMaxRunes:    24000,
	}
	if cfg := config.TryGet(); cfg != nil {
		e := cfg.Executor
		if e.PlanPromptMaxSteps > 0 {
			lim.promptMaxSteps = e.PlanPromptMaxSteps
		}
		if e.PlanMaxStepsPerPlan > 0 {
			lim.maxStepsPerPlan = e.PlanMaxStepsPerPlan
		}
		if e.PlanMaxDispatchPerTurn > 0 {
			lim.maxDispatchPerTurn = e.PlanMaxDispatchPerTurn
		}
		if e.PlanMaxAdjustPerStep > 0 {
			lim.maxAdjustPerStep = e.PlanMaxAdjustPerStep
		}
		if e.PlanResultSummaryMaxRunes > 0 {
			lim.resultSummaryMaxRunes = e.PlanResultSummaryMaxRunes
		}
		if e.PlanStepDetailMaxRunes > 0 {
			lim.stepDetailMaxRunes = e.PlanStepDetailMaxRunes
		}
	}
	return lim
}

// StepExecutor 执行 Agent 单步调用（由 BehaviorAgent 实现）。
type StepExecutor interface {
	Process(ctx context.Context, args ...interface{}) ([]interface{}, error)
}

// PlanAgent 负责需求拆分、TodoList 持久化、按步下发执行 Agent 与失败后的计划调节。
type PlanAgent struct {
	ModelKey       string
	Model          *prefrontalCortex.Mode
	Executor       StepExecutor
	SimpleExecutor StepExecutor
	Session        *sessionmemory.Manager
}

// NewPlanAgent 构造 PlanAgent（无 MCP/技能执行工具，编排由代码驱动）。
func NewPlanAgent(modelKey string, exec StepExecutor, simpleExec ...StepExecutor) (*PlanAgent, error) {
	if exec == nil {
		return nil, fmt.Errorf("plan agent: nil step executor")
	}
	m, ok := manager.ModelManager.ModelMap[modelKey]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelKey)
	}
	mode, ok := m.(*prefrontalCortex.Mode)
	if !ok {
		return nil, fmt.Errorf("plan agent requires *prefrontalCortex.Mode")
	}
	var sess *sessionmemory.Manager
	if cfg := config.TryGet(); cfg != nil {
		var err error
		sess, err = sessionmemory.NewFromConfig(cfg, mode)
		if err != nil {
			return nil, fmt.Errorf("plan session memory: %w", err)
		}
	}
	var simple StepExecutor
	if len(simpleExec) > 0 {
		simple = simpleExec[0]
	}
	return &PlanAgent{ModelKey: modelKey, Model: mode, Executor: exec, SimpleExecutor: simple, Session: sess}, nil
}

func (pa *PlanAgent) ReportActionResult(skillName string, out []interface{}, err error) {}

func (pa *PlanAgent) StartListening(ctx context.Context) {
	// PlanAgent 编排为同步 Process 循环，无需黑板监听。
}

// Process 一次用户诉求：创建独立 TodoList → 拆分 → 逐步下发 BehaviorAgent → 更新文件。
func (pa *PlanAgent) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	query, err := firstStringArg(args...)
	if err != nil {
		return nil, err
	}
	ctx = runcontrol.WithUserQuery(ctx, query)
	runcontrol.BeginPlanOrchestration()
	defer runcontrol.EndPlanOrchestration()

	doc := &todolist.Document{
		ID:              todolist.NewID(query),
		UserRequirement: query,
		Status:          todolist.PlanActive,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := pa.generateInitialPlan(ctx, doc); err != nil {
		return nil, fmt.Errorf("生成计划: %w", err)
	}
	if err := todolist.Save(doc); err != nil {
		return nil, fmt.Errorf("保存计划: %w", err)
	}

	route := memoryhook.Default().DecideRoute(ctx, memoryhook.RouteInput{
		Document:            doc,
		SimpleExecutorReady: pa.SimpleExecutor != nil,
	})
	fmt.Printf("[plan/memoryhook] 路由判定 use_simple=%v skip=%q matched=%v confidence=%.2f simple_ready=%v max_tier=%d\n",
		route.UseSimple,
		route.SkipReason,
		route.Experience.Matched,
		route.Experience.Confidence,
		pa.SimpleExecutor != nil,
		maxDocumentTier(doc),
	)
	if route.UseSimple {
		fmt.Println("[plan/memoryhook] → 下发 Exec-Simple episode")
		if handled, final, err := pa.runExecSimpleEpisode(ctx, doc, route.Experience); err != nil {
			return nil, err
		} else if handled {
			if pa.Session != nil {
				pa.Session.RecordTurn(ctx, query, stripPlanMetaFooter(final))
			}
			return []interface{}{final}, nil
		}
	}

	final, err := pa.runConservativeExecLoop(ctx, doc)
	if err != nil {
		return nil, err
	}
	if pa.Session != nil {
		pa.Session.RecordTurn(ctx, query, stripPlanMetaFooter(final))
	}
	return []interface{}{final}, nil
}

func (pa *PlanAgent) runConservativeExecLoop(ctx context.Context, doc *todolist.Document) (string, error) {
	if doc != nil && strings.TrimSpace(doc.ExecutionMode) == "" {
		doc.ExecutionMode = "exec"
	}
	limits := getPlanOrchestrationLimits()
	dispatches := 0
	for doc.Status == todolist.PlanActive && dispatches < limits.maxDispatchPerTurn {
		idx := doc.NextPending()
		if idx < 0 {
			if doc.AllTerminal() {
				doc.Status = todolist.PlanCompleted
				_ = todolist.Save(doc)
				break
			}
			doc.Status = todolist.PlanBlocked
			doc.BlockedReason = "无待执行步骤但计划未全部完成"
			_ = todolist.Save(doc)
			break
		}

		step := &doc.Steps[idx]
		step.Status = todolist.StepRunning
		step.Attempts++
		step.UpdatedAt = time.Now()
		cmd := buildStepCommand(doc, step, idx)
		doc.AppendFeedback(idx, "dispatch", cmd)
		_ = todolist.Save(doc)

		dispatches++
		stepCtx := runcontrol.WithPlanStepExecution(ctx)
		results, execErr := pa.Executor.Process(stepCtx, cmd)
		raw := ""
		if len(results) > 0 {
			raw = fmt.Sprintf("%v", results[0])
		}
		if execErr == nil {
			planstep.ReconcileSkillStepAfterRun(stepCtx, &raw)
		}
		outcome, summary, rep := classifyStepOutcome(stepCtx, raw, execErr, doc.UserRequirement, step.Tier)
		doc.AppendFeedback(idx, "result", summary)
		step.UpdatedAt = time.Now()

		switch outcome {
		case todolist.StepCompleted:
			step.Status = todolist.StepCompleted
			userDisplay := delivery.ResolveStepDisplay(rep.Summary, rep.UserVisible)
			todolist.RecordStepOutcome(step, rep.Summary, userDisplay, rep.Artifacts, rep.ToolsCalled)
			pa.maybePublishStepProgress(doc, idx, step)
			_ = todolist.Save(doc)
			continue
		case todolist.StepFailed:
			step.Status = todolist.StepFailed
			_ = todolist.Save(doc)
			if step.Attempts >= limits.maxAdjustPerStep {
				if blocked, msg := pa.escalateToUser(ctx, doc, idx, summary); blocked {
					_ = todolist.Save(doc)
					return msg, nil
				}
				continue
			}
			if err := pa.adjustPlanAfterFailure(ctx, doc, idx, summary); err != nil {
				return "", fmt.Errorf("计划调节: %w", err)
			}
			_ = todolist.Save(doc)
			continue
		}
	}

	final := pa.buildUserFacingReply(ctx, doc)
	return final, nil
}

func (pa *PlanAgent) runExecSimpleEpisode(ctx context.Context, doc *todolist.Document, exp memoryhook.Experience) (handled bool, final string, err error) {
	doc.ExecutionMode = "simple"
	if len(doc.Steps) > 0 {
		doc.AppendFeedback(0, "dispatch", fmt.Sprintf("下发 Exec-Simple episode：memory confidence=%.2f", exp.Confidence))
	}
	if err := todolist.Save(doc); err != nil {
		return false, "", err
	}

	simpleCtx := runcontrol.WithPlanSimpleExecution(ctx)
	cmd := buildSimpleEpisodeCommand(doc, exp)
	results, execErr := pa.SimpleExecutor.Process(simpleCtx, cmd)
	raw := ""
	if len(results) > 0 {
		raw = fmt.Sprintf("%v", results[0])
	}
	outcome, summary, rep := classifySimpleOutcome(simpleCtx, raw, execErr, maxDocumentTier(doc))
	if len(doc.Steps) > 0 {
		doc.AppendFeedback(0, "result", summary)
	}

	if outcome == todolist.StepCompleted {
		applySimpleSuccess(doc, rep)
		if err := todolist.Save(doc); err != nil {
			return false, "", err
		}
		return true, pa.buildUserFacingReply(ctx, doc), nil
	}

	// F2-6：simple 失败后不在原 simple episode 内硬扛，保留失败记录并新开保守 TodoList 交给逐步 Exec。
	doc.Status = todolist.PlanBlocked
	doc.BlockedReason = "Exec-Simple 快路径失败，已降级为阶段一逐步 Exec。失败摘要：" + summary
	_ = todolist.Save(doc)

	fallback := &todolist.Document{
		ID:              todolist.NewID(doc.UserRequirement + "-exec"),
		UserRequirement: doc.UserRequirement,
		Status:          todolist.PlanActive,
		ExecutionMode:   "exec",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := pa.generateInitialPlan(ctx, fallback); err != nil {
		return false, "", fmt.Errorf("simple 失败后生成保守计划: %w", err)
	}
	if len(fallback.Steps) > 0 {
		fallback.AppendFeedback(0, "adjust", "由 Exec-Simple 失败降级创建；失败摘要: "+summary)
	}
	if err := todolist.Save(fallback); err != nil {
		return false, "", err
	}
	final, err = pa.runConservativeExecLoop(ctx, fallback)
	if err != nil {
		return false, "", err
	}
	return true, final, nil
}

func buildSimpleEpisodeCommand(doc *todolist.Document, exp memoryhook.Experience) string {
	var b strings.Builder
	b.WriteString("【PlanAgent Exec-Simple Episode】\n")
	b.WriteString(fmt.Sprintf("计划ID: %s\n执行模式: simple\n", doc.ID))
	b.WriteString(fmt.Sprintf("用户总需求: %s\n", doc.UserRequirement))
	if strings.TrimSpace(doc.Summary) != "" {
		b.WriteString("需求摘要: " + doc.Summary + "\n")
	}
	b.WriteString(fmt.Sprintf("\n【Memory MCP 经验参考】confidence=%.2f\n", exp.Confidence))
	if strings.TrimSpace(exp.Summary) != "" {
		b.WriteString("经验摘要: " + exp.Summary + "\n")
	}
	if strings.TrimSpace(exp.PathHint) != "" {
		b.WriteString("成功路径提示: " + exp.PathHint + "\n")
	}
	b.WriteString("\n【TodoList-simple】\n")
	for i, s := range doc.Steps {
		b.WriteString(fmt.Sprintf("%d. [%s] %s | tier=%d\n", i+1, s.ID, s.Title, s.Tier))
		if strings.TrimSpace(s.Instruction) != "" {
			b.WriteString("   任务: " + s.Instruction + "\n")
		}
		if len(s.CapabilityHints) > 0 {
			b.WriteString("   能力提示: " + strings.Join(s.CapabilityHints, ", ") + "\n")
		}
	}
	b.WriteString("\n【执行规则】\n")
	b.WriteString("- 在一个连续 episode 内完成上面的压缩步骤链，可连续调用多个 MCP/技能。\n")
	b.WriteString("- episode 期间不要逐步回报 Plan；仅最终调用一次 report_step_result。\n")
	b.WriteString("- 成功：status=ok，并给出 summary、artifacts、tools_called。\n")
	b.WriteString("- 失败：status=fail，说明路径级错误；不要在 simple episode 内无限重试，Plan 会降级逐步 Exec。\n")
	b.WriteString("- 禁止 update_task_dashboard；禁止编造工具结果。\n")
	b.WriteString("- Soul/Memory 检索块仅供参考；目录/列表类任务以本次 filesystem 等工具返回为准。\n")
	return b.String()
}

func classifySimpleOutcome(ctx context.Context, raw string, execErr error, episodeTier int) (todolist.StepStatus, string, runcontrol.StepReport) {
	limits := getPlanOrchestrationLimits()
	var emptyRep runcontrol.StepReport
	if execErr != nil {
		return todolist.StepFailed, truncateRunesPlan(execErr.Error(), limits.resultSummaryMaxRunes), emptyRep
	}
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	rep, ok := runcontrol.TakeStepReport(turnID)
	if !ok {
		trim := strings.TrimSpace(raw)
		if trim == "" {
			return todolist.StepFailed, "Exec-Simple 无输出且未提交 report_step_result", emptyRep
		}
		return todolist.StepFailed, "Exec-Simple 未提交 episode 级 report_step_result", emptyRep
	}
	if strings.TrimSpace(rep.UserVisible) == "" {
		if uv := strings.TrimSpace(raw); uv != "" {
			rep.UserVisible = raw
		}
	}
	summary := truncateRunesPlan(cleanBehaviorResultText(rep.Summary), limits.resultSummaryMaxRunes)
	if strings.EqualFold(strings.TrimSpace(rep.Status), "ok") {
		v := verify.Gate(rep, episodeTier)
		if !v.Passed {
			msg := "Exec-Simple 验收未通过"
			if len(v.Failures) > 0 {
				msg = strings.Join(v.Failures, "; ")
			}
			return todolist.StepFailed, truncateRunesPlan(msg, limits.resultSummaryMaxRunes), rep
		}
		return todolist.StepCompleted, summary, rep
	}
	return todolist.StepFailed, summary, rep
}

func applySimpleSuccess(doc *todolist.Document, rep runcontrol.StepReport) {
	doc.Status = todolist.PlanCompleted
	now := time.Now()
	for i := range doc.Steps {
		s := &doc.Steps[i]
		s.UpdatedAt = now
		if i == 0 {
			s.Status = todolist.StepCompleted
			userDisplay := delivery.ResolveStepDisplay(rep.Summary, rep.UserVisible)
			todolist.RecordStepOutcome(s, rep.Summary, userDisplay, rep.Artifacts, rep.ToolsCalled)
			s.Feedback = append(s.Feedback, todolist.FeedbackEntry{At: now, Phase: "result", Summary: "Exec-Simple episode 成功完成"})
			continue
		}
		if s.Status != todolist.StepCompleted {
			s.Status = todolist.StepSkipped
			s.Feedback = append(s.Feedback, todolist.FeedbackEntry{At: now, Phase: "result", Summary: "Exec-Simple episode 已整体完成，本细分步骤不再逐步下发"})
		}
	}
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

func buildStepCommand(doc *todolist.Document, step *todolist.Step, idx int) string {
	var b strings.Builder
	b.WriteString("【PlanAgent 单步执行】\n")
	b.WriteString(fmt.Sprintf("计划ID: %s\n", doc.ID))
	b.WriteString(fmt.Sprintf("步骤: %d/%d | id=%s\n", idx+1, len(doc.Steps), step.ID))
	if step.Tier > 0 {
		b.WriteString(fmt.Sprintf("验收等级 tier: %d（1=轻 2=标准 3=重）\n", step.Tier))
	}
	b.WriteString(fmt.Sprintf("标题: %s\n", step.Title))
	b.WriteString(fmt.Sprintf("本步任务: %s\n", step.Instruction))
	if len(step.CapabilityHints) > 0 {
		b.WriteString("能力提示（名称级）: " + strings.Join(step.CapabilityHints, ", ") + "\n")
	}
	if road := todolist.FormatRoadmapForExec(doc, idx); road != "" {
		b.WriteString("\n【已完成步骤路标】（请据此衔接，勿重复已完成工作；需要详情时读取产出文件）\n")
		b.WriteString(road)
	}
	b.WriteString(fmt.Sprintf("\n用户总需求: %s\n", doc.UserRequirement))
	if step.Tier <= 1 {
		b.WriteString("约束: 本步为轻量/纯文本；仅完成本步骤。仅需 report_step_result，tools_called 可为 []。\n")
	} else {
		b.WriteString("约束: 仅完成本步骤，勿自行规划后续步骤；须真实调用本步所需 MCP/技能后再汇报。\n")
	}
	b.WriteString("交付（必须）：调用工具 report_step_result，示例：\n")
	if step.Tier <= 1 {
		b.WriteString(`  {"status":"ok","summary":"本步结论…","artifacts":[],"tools_called":[]}` + "\n")
	} else {
		b.WriteString(`  {"status":"ok","summary":"本步结论…","artifacts":["WorkSpace/…"],"tools_called":["SetExecutorStep"]}` + "\n")
	}
	b.WriteString("  status=fail 表示未完成。若调用了 SetExecutorStep，须等待真实技能结果后再标 ok。\n")
	b.WriteString("  若声明 artifacts 路径，下一步将读取该文件；占位/空文件将导致本步验收失败。\n")
	b.WriteString("禁止 update_task_dashboard；勿输出多步 ToolPlan JSON。\n")
	if stepNeedsBuiltinSkill(step) {
		b.WriteString("若本步调用 SetExecutorStep：须等待技能返回真实结果后再总结，禁止在仅收到「已接收后台执行」时当作完成。\n")
	}
	return b.String()
}

func stepNeedsBuiltinSkill(step *todolist.Step) bool {
	if step == nil {
		return false
	}
	for _, h := range step.CapabilityHints {
		h = strings.TrimSpace(h)
		if h != "" && !strings.Contains(h, "__") {
			return true
		}
	}
	return false
}

func classifyStepOutcome(ctx context.Context, raw string, execErr error, userRequirement string, stepTier int) (todolist.StepStatus, string, runcontrol.StepReport) {
	limits := getPlanOrchestrationLimits()
	var emptyRep runcontrol.StepReport
	if execErr != nil {
		return todolist.StepFailed, execErr.Error(), emptyRep
	}
	if runcontrol.IsPlanStepExecution(ctx) {
		turnID, _ := runcontrol.TurnMetaFromContext(ctx)
		if rep, ok := runcontrol.TakeStepReport(turnID); ok {
			if strings.TrimSpace(rep.UserVisible) == "" {
				if uv := strings.TrimSpace(raw); uv != "" && !strings.HasPrefix(uv, "（丘脑") {
					rep.UserVisible = raw
				}
			}
			summary := truncateRunesPlan(cleanBehaviorResultText(rep.Summary), limits.resultSummaryMaxRunes)
			if uv := strings.TrimSpace(rep.UserVisible); uv != "" {
				rep.UserVisible = truncateRunesPlan(cleanBehaviorResultText(uv), limits.stepDetailMaxRunes)
			}
			if strings.EqualFold(strings.TrimSpace(rep.Status), "ok") {
				v := verify.Gate(rep, stepTier)
				if !v.Passed {
					msg := "验收未通过"
					if len(v.Failures) > 0 {
						msg = strings.Join(v.Failures, "; ")
					}
					return todolist.StepFailed, truncateRunesPlan(msg, limits.resultSummaryMaxRunes), rep
				}
				return todolist.StepCompleted, summary, rep
			}
			return todolist.StepFailed, summary, rep
		}
		trim := strings.TrimSpace(raw)
		if trim == "" {
			return todolist.StepFailed, "执行 Agent 无输出且未提交 report_step_result", emptyRep
		}
		return todolist.StepFailed, "执行 Agent 未提交 report_step_result（Plan 单步必填）", emptyRep
	}

	summary := truncateRunesPlan(cleanBehaviorResultText(raw), limits.resultSummaryMaxRunes)
	trim := strings.TrimSpace(raw)
	if trim == "" {
		return todolist.StepFailed, "执行 Agent 无输出", emptyRep
	}
	if matched, _ := regexp.MatchString(planStepFailKeywords, raw); matched {
		return todolist.StepFailed, summary, emptyRep
	}
	return todolist.StepCompleted, summary, emptyRep
}

func (pa *PlanAgent) generateInitialPlan(ctx context.Context, doc *todolist.Document) error {
	limits := getPlanOrchestrationLimits()
	maxSteps := limits.promptMaxSteps
	if maxSteps > limits.maxStepsPerPlan {
		maxSteps = limits.maxStepsPerPlan
	}

	overview := capabilities.BuildPlanCapabilityOverview()
	prompt := `你是 PlanAgent：只做需求理解与步骤拆分，不执行具体 MCP/SKILL。
根据用户诉求与能力体系概览，由你动态判断需要几步（寒暄/致谢通常 1 步且 instruction 写明勿调工具；复杂任务再拆多步）。
输出 JSON（不要其它说明）：
{
  "summary": "一句话需求摘要",
  "steps": [
    {
      "id": "1",
      "title": "步骤标题",
      "instruction": "给执行 Agent 的可执行描述（单步、可验证）",
      "capability_hints": ["可选：MCP公开名或内置SKILL注册名"],
      "tier": 1
    }
  ]
}
要求：步骤数 1-` + fmt.Sprintf("%d", maxSteps) + `；每步可独立验收；只使用概览中出现的能力名称；不要写工具入参/Schema。
每步可选 tier：1=纯文本/寒暄/基于上一步归纳（instruction 写明勿调工具），2=标准，3=重。纯归纳步必须 tier=1，勿标 tier=2。
勿为与诉求无关的能力拆步（例如纯寒暄不要拆 MCP/发邮件/摄像头）。
若用户使用「他们/这些/上面/刚才」等指代，须结合【近期对话】与【相关记忆】解析指代对象，优先延续上一轮话题拆步，勿无故要求用户澄清。
能力列表/详情类追问：应拆步让执行 Agent 调用 list_agent_capabilities 或 get_capability_details，勿在 instruction 里预写答案后禁止调工具。

` + soulhook.ReferenceOnlyNotice

	user := fmt.Sprintf("用户诉求:\n%s\n\n%s\n\n当前计划文件将保存为: %s.json", doc.UserRequirement, overview, doc.ID)
	raw, err := pa.chatJSON(ctx, prompt, user, doc.UserRequirement)
	if err != nil {
		return err
	}
	var plan planJSON
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return fmt.Errorf("解析计划 JSON: %w", err)
	}
	doc.Summary = strings.TrimSpace(plan.Summary)
	if doc.Summary == "" {
		doc.Summary = truncateRunesPlan(doc.UserRequirement, 120)
	}
	now := time.Now()
	cap := maxSteps
	if cap > limits.maxStepsPerPlan {
		cap = limits.maxStepsPerPlan
	}
	for i, s := range plan.Steps {
		if i >= cap {
			break
		}
		id := strings.TrimSpace(s.ID)
		if id == "" {
			id = fmt.Sprintf("%d", i+1)
		}
		title := strings.TrimSpace(s.Title)
		instr := strings.TrimSpace(s.Instruction)
		doc.Steps = append(doc.Steps, todolist.Step{
			ID:              id,
			Title:           title,
			Instruction:     instr,
			CapabilityHints: s.CapabilityHints,
			Tier:            stepmeta.ResolveTier(s.Tier, title, instr, s.CapabilityHints),
			Status:          todolist.StepPending,
			UpdatedAt:       now,
		})
		doc.AppendFeedback(len(doc.Steps)-1, "create", "计划创建")
	}
	if len(doc.Steps) == 0 {
		doc.Steps = []todolist.Step{{
			ID:          "1",
			Title:       "执行用户诉求",
			Instruction: doc.UserRequirement,
			Tier:        stepmeta.InferTier("执行用户诉求", doc.UserRequirement, nil),
			Status:      todolist.StepPending,
			UpdatedAt:   now,
		}}
	}
	sanitizeDocumentCapabilityHints(doc)
	sanitizeDocumentTiers(doc)
	return nil
}

func (pa *PlanAgent) adjustPlanAfterFailure(ctx context.Context, doc *todolist.Document, failedIdx int, failSummary string) error {
	prompt := `你是 PlanAgent：根据失败反馈调节 TodoList（不执行工具）。
输出 JSON：
{
  "action": "retry|skip|replace_steps|block",
  "reason": "简短说明",
  "new_steps": [ 仅 action=replace_steps 时填写；每项为对象，含 id/title/instruction/capability_hints/tier，勿用纯字符串数组 ]
}
- retry: 保持步骤，将失败步改回 pending 以便重试
- skip: 将失败步标为 skipped
- replace_steps: 用 new_steps 替换从失败步起的后续（保留已完成步）
- block: 过于复杂，需用户决策

` + soulhook.ReferenceOnlyNotice

	user := fmt.Sprintf("失败步骤索引(1-based): %d\n失败摘要: %s\n\n当前计划:\n%s",
		failedIdx+1, failSummary, todolist.FormatForPrompt(doc))
	raw, err := pa.chatJSON(ctx, prompt, user, doc.UserRequirement)
	if err != nil {
		return err
	}
	adj, parseErr := parsePlanAdjustJSON(raw)
	if parseErr != nil {
		pa.applyPlanAdjustFallbackRetry(doc, failedIdx, parseErr)
		return nil
	}
	if err := pa.applyParsedPlanAdjust(doc, failedIdx, adj); err != nil {
		pa.applyPlanAdjustFallbackRetry(doc, failedIdx, err)
	}
	return nil
}

// applyPlanAdjustFallbackRetry 调节 JSON 无法解析或 replace_steps 无效时，兜底为 retry，避免整轮 Process 失败。
func (pa *PlanAgent) applyPlanAdjustFallbackRetry(doc *todolist.Document, failedIdx int, cause error) {
	if doc == nil || failedIdx < 0 || failedIdx >= len(doc.Steps) {
		return
	}
	doc.Steps[failedIdx].Status = todolist.StepPending
	msg := "fallback retry"
	if cause != nil {
		msg = fmt.Sprintf("fallback retry（调节 JSON 兜底: %v）", cause)
	}
	doc.AppendFeedback(failedIdx, "adjust", msg)
}

func (pa *PlanAgent) applyParsedPlanAdjust(doc *todolist.Document, failedIdx int, adj planAdjustJSON) error {
	if doc == nil || failedIdx < 0 || failedIdx >= len(doc.Steps) {
		return fmt.Errorf("invalid plan adjust target")
	}
	switch strings.ToLower(strings.TrimSpace(adj.Action)) {
	case "retry":
		doc.Steps[failedIdx].Status = todolist.StepPending
		doc.AppendFeedback(failedIdx, "adjust", adj.Reason)
	case "skip":
		doc.Steps[failedIdx].Status = todolist.StepSkipped
		doc.AppendFeedback(failedIdx, "adjust", "skip: "+adj.Reason)
	case "replace_steps":
		if len(adj.NewSteps) == 0 {
			return fmt.Errorf("replace_steps 但 new_steps 为空")
		}
		keep := doc.Steps[:failedIdx]
		now := time.Now()
		for i, s := range adj.NewSteps {
			id := strings.TrimSpace(s.ID)
			if id == "" {
				id = fmt.Sprintf("r%d-%d", failedIdx+1, i+1)
			}
			title := strings.TrimSpace(s.Title)
			instr := strings.TrimSpace(s.Instruction)
			if instr == "" {
				return fmt.Errorf("replace_steps 中第 %d 步缺少 instruction", i+1)
			}
			keep = append(keep, todolist.Step{
				ID:              id,
				Title:           title,
				Instruction:     instr,
				CapabilityHints: s.CapabilityHints,
				Tier:            stepmeta.ResolveTier(s.Tier, title, instr, s.CapabilityHints),
				Status:          todolist.StepPending,
				UpdatedAt:       now,
			})
		}
		doc.Steps = keep
		sanitizeDocumentCapabilityHints(doc)
		sanitizeDocumentTiers(doc)
		doc.AppendFeedback(failedIdx, "adjust", "replace: "+adj.Reason)
	case "block":
		doc.Status = todolist.PlanBlocked
		doc.BlockedReason = adj.Reason
		doc.Steps[failedIdx].Status = todolist.StepBlocked
	default:
		doc.Steps[failedIdx].Status = todolist.StepPending
		doc.AppendFeedback(failedIdx, "adjust", "default retry: "+adj.Reason)
	}
	return nil
}

func (pa *PlanAgent) escalateToUser(ctx context.Context, doc *todolist.Document, failedIdx int, failSummary string) (blocked bool, msg string) {
	doc.Status = todolist.PlanBlocked
	doc.BlockedReason = fmt.Sprintf("步骤「%s」多次失败，需用户决策", doc.Steps[failedIdx].Title)
	doc.Steps[failedIdx].Status = todolist.StepBlocked
	doc.AppendFeedback(failedIdx, "escalate", failSummary)

	body := pa.buildUserFacingReply(ctx, doc)
	return true, body
}

// buildUserFacingReply 汇总各步真实执行结果，作为提交给前端/用户的正文（非仅计划状态清单）。
func (pa *PlanAgent) buildUserFacingReply(ctx context.Context, doc *todolist.Document) string {
	results := doc.CollectStepResults()
	var body string

	switch {
	case doc.Status == todolist.PlanBlocked:
		body = strings.TrimSpace(doc.BlockedReason)
		if last := doc.LastNonEmptyResult(); last != "" {
			if body != "" {
				body += "\n\n"
			}
			body += cleanBehaviorResultText(last)
		}
	case len(results) == 0:
		if last := doc.LastNonEmptyResult(); last != "" {
			body = cleanBehaviorResultText(last)
		} else {
			body = "任务未产生可展示的执行结果，请查看日志或重试。"
		}
	case len(results) == 1:
		body = cleanBehaviorResultText(results[0])
		if pa.shouldSynthesizeSingleStep(body) && delivery.ShouldSynthesizeFinalReply(body, pa.synthesizeMinRunes()) {
			if syn := pa.synthesizeUserReply(ctx, doc.UserRequirement, results); strings.TrimSpace(syn) != "" {
				body = syn
			}
		}
	default:
		body = pa.synthesizeUserReply(ctx, doc.UserRequirement, results)
		if strings.TrimSpace(body) == "" {
			body = strings.Join(cleanBehaviorResults(results), "\n\n---\n\n")
		}
	}

	body = strings.TrimSpace(body)
	if body == "" {
		body = "任务已结束，但未生成可展示文本。"
	}
	footer := planMetaFooter(doc)
	if runcontrol.SynthesizeStreamed(ctx) {
		turnID, _ := runcontrol.TurnMetaFromContext(ctx)
		msgID := runcontrol.SynthesizeStreamMessageID(ctx)
		if turnID != "" && msgID != "" {
			outputbus.PublishStreamFinal("计划编排", turnID, msgID, "\n\n"+footer)
		}
	}
	return body + "\n\n" + footer
}

func (pa *PlanAgent) synthesizeStreamEnabled() bool {
	cfg := config.TryGet()
	if cfg == nil {
		return false
	}
	return cfg.Executor.PlanDelivery.StreamSynthesizeReply
}

func (pa *PlanAgent) synthesizeUserReply(ctx context.Context, requirement string, stepResults []string) string {
	var b strings.Builder
	b.WriteString("用户诉求:\n")
	b.WriteString(requirement)
	b.WriteString("\n\n各步骤执行结果:\n")
	for i, r := range stepResults {
		b.WriteString(fmt.Sprintf("\n--- 步骤 %d ---\n%s", i+1, cleanBehaviorResultText(r)))
	}
	system := `你是交付助手。根据「用户诉求」与「各步骤执行结果」写一段直接给用户看的最终回复。
要求：包含具体数据、路径、表名、文件、错误原因等；语言简洁；不要只列计划/步骤状态；不要提及 TodoList 文件；不要编造结果中不存在的内容。
只输出给用户看的正文，不要 JSON。`
	msgs := []llms.MessageContent{
		{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextPart(system)}},
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextPart(b.String())}},
	}

	if !pa.synthesizeStreamEnabled() {
		resp, err := pa.Model.GenerateContent(ctx, msgs)
		if err != nil || resp == nil || len(resp.Choices) == 0 {
			return ""
		}
		return strings.TrimSpace(resp.Choices[0].Content)
	}

	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	if turnID == "" {
		turnID = fmt.Sprintf("plan-%d", time.Now().UnixNano())
	}
	messageID := turnID + "-synthesize"
	ctx = runcontrol.BeginSynthesizeStream(ctx, turnID, messageID)

	streamedAny := false
	onDelta := func(chunk string) error {
		streamedAny = true
		outputbus.PublishStreamDelta("计划编排", turnID, messageID, chunk)
		return nil
	}
	text, err := pa.Model.GenerateContentStream(ctx, msgs, onDelta)
	if err != nil {
		fmt.Printf("[plan] synthesizeUserReply 流式失败，回退非流式: %v\n", err)
		resp, gerr := pa.Model.GenerateContent(ctx, msgs)
		if gerr != nil || resp == nil || len(resp.Choices) == 0 {
			runcontrol.ClearSynthesizeStream(turnID)
			return ""
		}
		fallback := strings.TrimSpace(resp.Choices[0].Content)
		if fallback == "" {
			runcontrol.ClearSynthesizeStream(turnID)
			return ""
		}
		if streamedAny {
			outputbus.PublishStreamFinal("计划编排", turnID, messageID, fallback)
			return fallback
		}
		runcontrol.ClearSynthesizeStream(turnID)
		return fallback
	}
	text = strings.TrimSpace(text)
	if text == "" {
		runcontrol.ClearSynthesizeStream(turnID)
		return ""
	}
	return text
}

func (pa *PlanAgent) shouldSynthesizeSingleStep(_ string) bool {
	cfg := config.TryGet()
	if cfg == nil {
		return true
	}
	return !cfg.Executor.PlanDelivery.DisableSingleStepSynthesize
}

func (pa *PlanAgent) synthesizeMinRunes() int {
	cfg := config.TryGet()
	if cfg == nil || cfg.Executor.PlanDelivery.SynthesizeMinRunes <= 0 {
		return 400
	}
	return cfg.Executor.PlanDelivery.SynthesizeMinRunes
}

func (pa *PlanAgent) maybePublishStepProgress(doc *todolist.Document, idx int, step *todolist.Step) {
	cfg := config.TryGet()
	if cfg == nil || !cfg.Executor.PlanDelivery.ProgressToPortal || doc == nil || step == nil {
		return
	}
	line := delivery.StepProgressLine(idx+1, len(doc.Steps), step.Title, step.ResultSummary, 280)
	outputbus.Publish("计划进度", line)
}

func planMetaFooter(doc *todolist.Document) string {
	path, _ := todolist.Path(doc.ID)
	return fmt.Sprintf("---\n（编排 %s · %s · %d 步 · 记录: %s）", doc.ID, doc.Status, len(doc.Steps), path)
}

func stripPlanMetaFooter(body string) string {
	const sep = "\n\n---\n（编排 "
	if i := strings.LastIndex(body, sep); i >= 0 {
		return strings.TrimSpace(body[:i])
	}
	return strings.TrimSpace(body)
}

func cleanBehaviorResults(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := cleanBehaviorResultText(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func cleanBehaviorResultText(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "Thought: I have the answer now.") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "Thought: I have the answer now."))
	}
	return s
}

func (pa *PlanAgent) chatJSON(ctx context.Context, system, user, recallQuery string) (string, error) {
	var msgs []llms.MessageContent
	if pa.Session != nil {
		ctxBlock := pa.Session.PrepareUserContext(ctx, recallQuery)
		msgs = pa.Session.BuildMessages(system, user, ctxBlock)
	} else {
		msgs = []llms.MessageContent{
			{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextPart(system)}},
			{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextPart(user)}},
		}
	}
	resp, err := pa.Model.GenerateContent(ctx, msgs)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", fmt.Errorf("模型无响应")
	}
	return extractJSONObject(resp.Choices[0].Content), nil
}

type planJSON struct {
	Summary string         `json:"summary"`
	Steps   []planStepJSON `json:"steps"`
}

type planStepJSON struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Instruction     string   `json:"instruction"`
	CapabilityHints []string `json:"capability_hints"`
	Tier            int      `json:"tier,omitempty"`
}

type planAdjustJSON struct {
	Action   string         `json:"action"`
	Reason   string         `json:"reason"`
	NewSteps []planStepJSON `json:"new_steps"`
}

var reJSONBlock = regexp.MustCompile("(?is)```(?:json)?\\s*([\\s\\S]*?)```")

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if m := reJSONBlock.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if inStr {
			if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

func sanitizeDocumentCapabilityHints(doc *todolist.Document) {
	if doc == nil {
		return
	}
	for i := range doc.Steps {
		cleaned, removed := capabilities.SanitizeCapabilityHints(doc.Steps[i].CapabilityHints)
		doc.Steps[i].CapabilityHints = cleaned
		if len(removed) > 0 {
			doc.AppendFeedback(i, "validate", "移除未知能力名: "+strings.Join(removed, ", "))
		}
	}
}

func sanitizeDocumentTiers(doc *todolist.Document) {
	if doc == nil {
		return
	}
	for i := range doc.Steps {
		s := &doc.Steps[i]
		resolved := stepmeta.ResolveTier(s.Tier, s.Title, s.Instruction, s.CapabilityHints)
		if resolved != s.Tier {
			doc.AppendFeedback(i, "validate", fmt.Sprintf("tier %d→%d（instruction 为纯文本/归纳步）", s.Tier, resolved))
			s.Tier = resolved
		}
	}
}

func truncateRunesPlan(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func firstStringArg(args ...interface{}) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("query is empty")
	}
	switch v := args[0].(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("query is empty")
		}
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
