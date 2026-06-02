package portal

import (
	"AgentTest/agent"
	"AgentTest/agent/runcontrol"
	"AgentTest/interaction"
	"AgentTest/outputbus"
	"AgentTest/plan/memoryhook"
	"AgentTest/plan/soulhook"
	"AgentTest/turnjournal"
	"AgentTest/userupload"
	"AgentTest/util/sendTopic"
	"context"
	"fmt"
	"strings"
)

// UnifiedOutputGateway 统一输出：控制台 + 广播给 Web SSE 等订阅方。
func UnifiedOutputGateway(source string, content interface{}) {
	fmt.Printf("\n[%s] ======= > %v\n", source, content)
	if turnID := runcontrol.CurrentTurnID(); turnID != "" {
		outputbus.PublishForTurn(source, turnID, content)
	} else {
		outputbus.Publish(source, content)
	}
}

// RunRouterTurn 用户主入口（经 interaction.Router：入站标注 + 回执绑定）。
// stagingID 非空时将暂存附件移至 WorkSpace/inbox/{turn_id}/。
func RunRouterTurn(ctx context.Context, input string, stagingID string) error {
	return interaction.Default().HandleTurn(ctx, interaction.TurnRequest{
		Channel:   interaction.ChannelWeb,
		Message:   input,
		StagingID: stagingID,
	})
}

// ProcessTurn 在已 BeginTurn 的 turnCtx 上执行编排（由 interaction.Router 调用）。
// routingPrefix 为【交互路由·本回合】块，将拼入 planInput。
func ProcessTurn(turnCtx context.Context, input string, stagingID string, routingPrefix string) error {
	input = strings.TrimSpace(input)
	stagingID = strings.TrimSpace(stagingID)
	manager := agent.GetManager()
	if manager == nil {
		return fmt.Errorf("agent manager 未初始化")
	}
	turnID, _ := runcontrol.TurnMetaFromContext(turnCtx)
	if turnID == "" {
		turnID = runcontrol.CurrentTurnID()
	}

	planAgent, ok := manager.Agents["planAgent"]
	if !ok || planAgent == nil {
		return processBehaviorFallback(turnCtx, input, stagingID, turnID, routingPrefix)
	}

	sendTopic.ResetFacadeDedup()
	baseInput, _ := enrichInputWithInbox(input, stagingID, turnID)
	soulHints := soulhook.Default().RetrieveTurnBeforeProcess(turnCtx, baseInput)
	memoryHints := memoryhook.Default().RetrieveTurnBeforeProcess(turnCtx, baseInput)
	planInput := soulhook.CombineTurnHints(baseInput, soulHints, memoryHints)
	planInput = interaction.PrefixPlanInput(routingPrefix, planInput)

	turnjournal.Begin(turnjournal.BeginInput{
		TurnID: turnID, UserInput: input, PlanInput: planInput, PathMode: "plan",
	})
	journalFin := turnjournal.FinalizeInput{TurnID: turnID, OutputSource: "计划编排"}
	defer func() { turnjournal.Finalize(journalFin) }()

	results, err := planAgent.Process(turnCtx, planInput)
	reply := ""
	if len(results) > 0 {
		reply = fmt.Sprintf("%v", results[0])
	}
	procErr := ""
	if err != nil {
		procErr = err.Error()
	}
	journalFin.Reply = reply
	journalFin.ProcessError = procErr
	storeSoulMemory(turnCtx, turnID, input, reply, procErr)

	if err != nil {
		UnifiedOutputGateway("系统异常", err.Error())
		return err
	}
	if reply != "" {
		streamed := runcontrol.SynthesizeStreamed(turnCtx)
		journalFin.Streamed = streamed
		if streamed {
			fmt.Printf("\n[计划编排] ======= > (已通过流式推送交付助手正文，跳过整包重复)\n")
			runcontrol.ClearSynthesizeStream(turnID)
		} else {
			UnifiedOutputGateway("计划编排", sendTopic.SanitizeFacadeText(reply))
		}
	}
	return nil
}

func processBehaviorFallback(turnCtx context.Context, input, stagingID, turnID, routingPrefix string) error {
	behaviorAgent, ok := agent.GetManager().Agents["behaviorAgent"]
	if !ok || behaviorAgent == nil {
		return fmt.Errorf("planAgent 与 behaviorAgent 均不可用")
	}
	baseInput, _ := enrichInputWithInbox(input, stagingID, turnID)
	planInput := interaction.PrefixPlanInput(routingPrefix, baseInput)
	turnjournal.Begin(turnjournal.BeginInput{
		TurnID: turnID, UserInput: input, PlanInput: planInput, PathMode: "behavior_fallback",
	})
	journalFin := turnjournal.FinalizeInput{TurnID: turnID, OutputSource: "行为编排"}
	defer func() { turnjournal.Finalize(journalFin) }()

	sendTopic.ResetFacadeDedup()
	results, err := behaviorAgent.Process(turnCtx, planInput)
	reply := ""
	if len(results) > 0 {
		reply = fmt.Sprintf("%v", results[0])
	}
	procErr := ""
	if err != nil {
		procErr = err.Error()
	}
	journalFin.Reply = reply
	journalFin.ProcessError = procErr
	storeSoulMemory(turnCtx, turnID, input, reply, procErr)
	if err != nil {
		UnifiedOutputGateway("系统异常", err.Error())
		return err
	}
	if reply != "" {
		UnifiedOutputGateway("行为编排", reply)
	}
	return nil
}

func storeSoulMemory(turnCtx context.Context, turnID, input, reply, procErr string) {
	ch := interaction.ChannelWeb
	if m, ok := runcontrol.InteractionMetaFromContext(turnCtx); ok && m.Channel != "" {
		ch = m.Channel
	}
	soulhook.Default().StoreTurnAfterProcess(turnCtx, soulhook.WebUITurnInput{
		TurnID:         turnID,
		UserInput:      input,
		AssistantReply: reply,
		ProcessError:   procErr,
		Channel:        ch,
	})
	memoryhook.Default().StoreTurnAfterProcess(turnCtx, memoryhook.TurnStoreInput{
		TurnID:         turnID,
		UserInput:      input,
		AssistantReply: reply,
		ProcessError:   procErr,
	})
}

func enrichInputWithInbox(input, stagingID, turnID string) (string, []userupload.Entry) {
	if stagingID == "" {
		return input, nil
	}
	entries, err := userupload.FinalizeStaging(stagingID, turnID)
	if err != nil {
		fmt.Printf("[inbox] 附件落盘失败 staging=%s turn=%s: %v\n", stagingID, turnID, err)
		return input, nil
	}
	block := userupload.FormatAttachmentsBlock(entries)
	if block == "" {
		return input, entries
	}
	if strings.TrimSpace(input) == "" {
		return block, entries
	}
	return input + "\n\n" + block, entries
}
