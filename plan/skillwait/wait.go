// Package skillwait 在 PlanAgent 单步中等待 SetExecutorStep 触发的异步技能完成。
package skillwait

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const DefaultTimeout = 3 * time.Minute

// NeedsWait 判断文本是否为 SetExecutorStep 工具回执（仅「已提交」、尚无技能结果）。
// 勿用宽泛子串匹配：模型在最终答复里复述「已接收」会误触发等待。
func NeedsWait(behaviorOutput string) bool {
	s := strings.TrimSpace(behaviorOutput)
	return strings.HasPrefix(s, "已接收：后台异步执行内置技能")
}

// Wait 阻塞直到当前回合的 skill 执行成功/失败或超时。返回可供用户展示的技能输出文本。
func Wait(ctx context.Context, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	if cached, ok := TakeCachedResult(turnID); ok {
		return cached, nil
	}
	stepCh := blackboard.GetInstance().Subscribe(blackboard.TopicExecStepEvent, 16)
	resultCh := blackboard.GetInstance().Subscribe(blackboard.TopicExecResult, 4)
	defer func() {
		// 订阅通道随进程存活；仅在本轮 wait 结束后不再读取
	}()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("等待内置技能执行超时（%s）", timeout)
		case msg := <-stepCh:
			if !matchTurn(turnID, msg.TurnID) {
				continue
			}
			payload, ok := msg.Payload.(map[string]interface{})
			if !ok {
				continue
			}
			typ, _ := payload["type"].(string)
			switch typ {
			case "success":
				return formatOutput(payload["output"]), nil
			case "error":
				errMsg, _ := payload["error"].(string)
				if errMsg == "" {
					errMsg = "技能执行失败"
				}
				return "", fmt.Errorf("%s", errMsg)
			}
		case msg := <-resultCh:
			// 兼容未带 TurnID 的 Publish（尽量只在本回合 wait 窗口内采用）
			if turnID != "" && msg.TurnID != "" && msg.TurnID != turnID {
				continue
			}
			return formatOutput(msg.Payload), nil
		}
	}
}

func matchTurn(expect, got string) bool {
	if expect == "" {
		return got == ""
	}
	return got == expect
}

func formatOutput(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case []interface{}:
		var parts []string
		for _, item := range x {
			if s := formatOutput(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(b)
	}
}
