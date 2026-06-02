package community

import (
	"AgentTest/agent"
	"AgentTest/agentWorkSpace/portal"
	"AgentTest/body/blackboard"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// ======================
// 数据结构
// ======================

type RegisterRequest struct {
	AgentID     string `json:"agent_id"`
	AgentType   string `json:"agent_type"`
	CallbackURL string `json:"callback_url"`
}

type CommunityInput struct {
	Content string `json:"content"`
}

// Agent 项目中
type OutputMessage struct {
	AgentID string `json:"agent_id"`
	Content string `json:"content"`
	Time    string `json:"time,omitempty"` // 加上这个
}

// ======================
// 注册函数
// ======================

func RegisterToCommunity(ctx context.Context, communityURL, agentID, agentType, callbackURL string) error {
	reqBody := RegisterRequest{
		AgentID:     agentID,
		AgentType:   agentType,
		CallbackURL: callbackURL,
	}

	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", communityURL+"/register", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Println("[CommunityClient] 注册成功")
	return nil
}

func SendOutputToCommunity(ctx context.Context, communityURL, agentID, content string) {
	msg := OutputMessage{
		AgentID: agentID,
		Content: content,
	}
	data, _ := json.Marshal(msg)

	req, _ := http.NewRequestWithContext(ctx, "POST", communityURL+"/output", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

// ======================
// 主函数
// ======================

func WorkStartCommunity() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	behaviorAgent.StartListening(ctx)

	communityURL := "http://localhost:8080"
	agentID := "interactiveAgent"
	agentPort := "8091"

	// ==================================================
	// 1️⃣ 新增：异步消息订阅流 (同步 WorkStart 的逻辑)
	// ==================================================
	// 订阅黑板上的输出信号
	portalOutputCh := blackboard.GetInstance().Subscribe(blackboard.TopicFacadeOutput, 10)

	go func() {
		fmt.Println("[Community] 已开启黑板异步消息转发...")
		for {
			select {
			case msg := <-portalOutputCh:
				// 捕获到异步上报（如：Agent 自发的反馈），转发给社区
				content := fmt.Sprintf("%v", msg.Payload)
				fmt.Println("[Agent] 转发异步消息至社区:", content)
				SendOutputToCommunity(ctx, communityURL, agentID, content)
			case <-ctx.Done():
				return
			}
		}
	}()

	// ======================
	// 2️⃣ 启动 Agent 的 HTTP Server
	// ======================
	mux := http.NewServeMux()

	mux.HandleFunc("/community/input", func(w http.ResponseWriter, r *http.Request) {
		var input CommunityInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			return
		}

		fmt.Println("[Agent] 收到社区输入:", input.Content)

		if err := portal.RunRouterTurn(ctx, input.Content, ""); err != nil {
			fmt.Println("RunRouterTurn error:", err)
			return
		}
	})

	go func() {
		fmt.Println("[Agent] HTTP Server 启动端口:", agentPort)
		http.ListenAndServe(":"+agentPort, mux)
	}()

	// ======================
	// 3️⃣ 注册到社区
	// ======================
	callbackURL := "http://localhost:" + agentPort + "/community/input"
	RegisterToCommunity(ctx, communityURL, agentID, "BehaviorAgent", callbackURL)

	fmt.Println("🚀 Agent 已接入社区（支持同步对话与异步订阅转发）")

	// ======================
	// 4️⃣ 退出监听
	// ======================
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("关闭 Agent...")
	case <-ctx.Done():
	}
}
