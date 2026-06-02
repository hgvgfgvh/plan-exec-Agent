package agent

import (
	_ "AgentTest/Init/InitStep1"
	"AgentTest/agent/agent"
	"AgentTest/agent/agentInterface"
	"AgentTest/config"
	"AgentTest/manager"
	"AgentTest/plan/memoryhook"
	"AgentTest/plan/soulhook"
	"fmt"
	"sync"
)

type AgentManager struct {
	mu     sync.RWMutex
	Agents map[string]agentInterface.AgentInterface
}

var mgmt *AgentManager

var initAgentsOnce sync.Once

// GetManager 返回已初始化的管理器；若 InitAgents 未成功调用则可能为 nil。
func GetManager() *AgentManager {
	return mgmt
}

// InitAgents 根据配置创建全部 Agent（须在 config.SetGlobal 之后、业务启动前调用一次）。
func InitAgents(cfg *config.App) error {
	var initErr error
	initAgentsOnce.Do(func() {
		if cfg == nil {
			initErr = fmt.Errorf("InitAgents: nil config")
			return
		}
		manager.InitModelsFromConfig(cfg)
		if err := memoryhook.InitFromConfig(cfg); err != nil {
			initErr = fmt.Errorf("plan memory hook: %w", err)
			return
		}
		if err := soulhook.InitFromConfig(cfg); err != nil {
			initErr = fmt.Errorf("plan soul hook: %w", err)
			return
		}
		fmt.Printf("[plan/memoryhook] 已挂载 provider=%s enabled=%v exec_simple=%v\n",
			memoryhook.Default().ProviderName(),
			cfg.PlanMemoryHook.Enabled,
			cfg.Executor.ExecSimpleEnabled,
		)
		fmt.Printf("[plan/soulhook] 已挂载 provider=%s enabled=%v\n",
			soulhook.Default().ProviderName(),
			cfg.PlanSoulHook.Enabled,
		)
		a := cfg.Agents
		e := cfg.ResolvedPaths()

		model := a.DefaultModel
		ragN := a.RAGRecallThreshold

		behaviorAgent, err := agent.NewBehaviorAgent(
			e.Abilities, model, e.Memory, ragN, a.BehaviorZeroState, e.Experience,
		)
		if err != nil {
			fmt.Println("behaviorAgent error:" + err.Error())
		}

		execSimpleAgent, err := agent.NewExecSimpleAgent(
			e.Abilities, model, e.Memory, ragN, e.Experience,
		)
		if err != nil {
			fmt.Println("[execSimpleAgent] 初始化失败（将无法走 Exec-Simple）: " + err.Error())
			execSimpleAgent = nil
		} else {
			fmt.Println("[execSimpleAgent] 已就绪")
		}

		baseAgent, err := agent.NewBaseAgent(model, e.Memory, ragN)
		if err != nil {
			fmt.Println("baseAgent error:" + err.Error())
		}

		interactiveAgent, err := agent.NewAffectiveInteractiveAgent(e.Soul, model, e.Memory, ragN)
		if err != nil {
			fmt.Println("interactiveAgent error:" + err.Error())
		}

		routerAgent, err := agent.NewRouterAgent(model, e.Memory, ragN)
		if err != nil {
			fmt.Println("routerAgent error:" + err.Error())
		}

		var simpleExec agent.StepExecutor
		if execSimpleAgent != nil {
			simpleExec = execSimpleAgent
		}
		planAgent, err := agent.NewPlanAgent(model, behaviorAgent, simpleExec)
		if err != nil {
			fmt.Println("planAgent error:" + err.Error())
		}

		agents := map[string]agentInterface.AgentInterface{
			"behaviorAgent":    behaviorAgent,
			"baseAgent":        baseAgent,
			"interactiveAgent": interactiveAgent,
			"routerAgent":      routerAgent,
			"planAgent":        planAgent,
		}
		if execSimpleAgent != nil {
			agents["execSimpleAgent"] = execSimpleAgent
		}
		mgmt = &AgentManager{Agents: agents}
	})
	return initErr
}
