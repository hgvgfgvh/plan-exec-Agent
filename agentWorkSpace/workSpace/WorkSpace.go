package workSpace

import (
	"AgentTest/agent"
	"AgentTest/agent/runcontrol"
	"AgentTest/agentWorkSpace/portal"
	"AgentTest/body/blackboard"
	"AgentTest/capabilities"
	"AgentTest/config"
	"AgentTest/runview"
	"AgentTest/skillpacks"
	"AgentTest/turnjournal"
	"AgentTest/webui"
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func WorkStart() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runcontrol.Boot(ctx)
	capabilities.ShutdownOnContext(ctx)
	skillpacks.StartWatcher(ctx, config.Get())
	turnjournal.StartSubscriber(ctx)
	runview.Start(ctx)

	manager := agent.GetManager()
	if manager == nil {
		fmt.Println("Agent Manager 初始化失败")
		return
	}

	behaviorAgent := manager.Agents["behaviorAgent"]
	if behaviorAgent == nil {
		fmt.Println("behaviorAgent 不可用")
		return
	}
	planAgent := manager.Agents["planAgent"]
	if planAgent == nil {
		fmt.Println("planAgent 不可用，将回退直通 Behavior")
	}

	// Behavior 异步 skill / 黑板；Plan 为主入口同步编排
	behaviorAgent.StartListening(ctx)
	if planAgent != nil {
		planAgent.StartListening(ctx)
	}

	fmt.Println("==================================================")
	fmt.Println("🚀 数字生命内核已启动 | 主入口：PlanAgent")
	fmt.Println("用户输入 → PlanAgent（拆分 TodoList）→ BehaviorAgent（逐步执行）")
	fmt.Println("TodoList 目录: WorkSpace/ToDoList/")
	fmt.Println("==================================================")

	cfg := config.Get()

	// --- Web UI（与内核同生命周期）---
	if cfg.Web.Enabled {
		go webui.Start(ctx, cfg)
	}

	// --- 1. 异步消息订阅流 ---
	// WebUI 通过「计划编排/计划进度」展示主输出；为避免出现额外的「答复/反馈」气泡，
	// Web 模式下不再把 facade.output 桥接到 SSE。非 Web（stdin）模式仍保留该通道。
	if !cfg.Web.Enabled {
		portalOutputCh := blackboard.GetInstance().Subscribe(blackboard.TopicFacadeOutput, 10)
		go func() {
			for {
				select {
				case msg := <-portalOutputCh:
					portal.UnifiedOutputGateway("反馈", msg.Payload)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// --- 2. 同步对话交互流（仅非 Web 模式使用 stdin）---
	if !cfg.Web.Enabled {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print("\nUser > ")
				if !scanner.Scan() {
					break
				}
				input := scanner.Text()
				if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
					fmt.Println("正在关闭脑中枢...")
					cancel()
					return
				}
				// RunRouterTurn 内部已 BeginTurn，勿重复调用以免取消刚建立的回合上下文
				_ = portal.RunRouterTurn(ctx, input, "")
			}
		}()
	} else {
		fmt.Println("[webui] 交互请使用浏览器；终端不再读取 stdin。")
	}

	// 3. 信号监听
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("\n接收到停机指令，正在保存记忆...")
		cancel()
	case <-ctx.Done():
	}
}
