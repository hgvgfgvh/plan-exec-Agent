package blackboard

// 全仓黑板主题集中常量。
//
// 命名约定：
//   - agent.<AgentName>.dispatch  : Router → 目标 Agent，「用户原始意图」首次投递（不带反思包装）。
//   - agent.<AgentName>.input     : Router → 目标 Agent，「反思链」转发的脑分区互通消息。
//   - agent.<AgentName>.output    : 目标 Agent → Router，向丘脑回报本分区动作/结论。
//   - facadeInteraction.output    : 任何分区 → 用户门面（终端/WebUI 等）。
//   - exec.*                      : 行为执行链的状态/事件/结果上报。
//   - agent.control.*             : 控制信号（中止等）。
//   - env.change                  : 环境/感知层变化上报。
//
// 所有发布/订阅请使用这些常量，禁止在业务代码里直接出现 topic 字面量；
// 改名只动这一处即可保证全仓一致。
const (
	// Affective（情感直接交互脑分区）
	TopicAffectiveDispatch = "agent.AffectiveInteractiveAgent.dispatch"
	TopicAffectiveInput    = "agent.AffectiveInteractiveAgent.input" // 原 ".iutput"（typo），已统一拼写
	TopicAffectiveOutput   = "agent.AffectiveInteractiveAgent.output"

	// Behavior（行为编排调度脑分区）
	TopicBehaviorInput  = "agent.BehaviorAgent.input"
	TopicBehaviorOutput = "agent.BehaviorAgent.output"

	// Portal / 用户门面
	TopicFacadeOutput = "facadeInteraction.output"

	// 执行链
	TopicExecStepEvent = "exec.step.event"
	TopicExecStatus    = "exec.status"
	TopicExecResult    = "exec.result"

	// 控制 / 环境
	TopicAgentAbort = "agent.control.abort"
	TopicEnvChange  = "env.change"
)
