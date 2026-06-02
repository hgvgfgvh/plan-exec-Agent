package executor

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/body/blackboard"
	"context"
	"fmt"
)

// ExecuteSkillTree 按深度优先执行技能树
// initialArgs: 初始输入参数
// 返回值: 最终叶子节点的执行结果，以及是否成功执行到终点
func ExecuteSkillTree(ctx context.Context, node *SkillNode, initialArgs []interface{}) ([]interface{}, bool, error) {
	// 【新增检测】在执行任何逻辑前，检查 Context 是否已中止
	select {
	case <-ctx.Done():
		fmt.Printf("[中止] 收到取消信号，停止执行技能: %s\n", node.Skill.Name())
		return nil, false, ctx.Err()
	default:
		// 继续执行
	}
	// --- 【新增：上报开始执行这一步】 ---
	//blackboard.GetInstance().Publish("exec.step.event", map[string]interface{}{
	//	"type":  "start",
	//	"skill": node.Skill.Name(),
	//	"args":  initialArgs,
	//})

	if node == nil {
		return nil, false, nil
	}

	// 1. 执行当前节点
	fmt.Printf("[执行中] Skill: %s\n", node.Skill.Name())
	results, err := node.Skill.Execute(ctx, initialArgs...)

	// 2. 如果当前节点执行报错，触发回退逻辑
	if err != nil {
		// --- 【新增：上报执行失败】 ---
		publishExecStepEvent(ctx, map[string]interface{}{
			"type":  "error",
			"skill": node.Skill.Name(),
			"error": err.Error(),
		})

		fmt.Printf("[执行失败] Skill: %s, 错误: %v, 准备尝试兄弟节点(如有)\n", node.Skill.Name(), err)
		return nil, false, err
	}
	// --- 【新增：上报执行成功及结果】 ---
	publishExecStepEvent(ctx, map[string]interface{}{
		"type":   "success",
		"skill":  node.Skill.Name(),
		"output": results,
	})

	// 3. 成功条件：如果没有子节点了，说明这条路径走通了
	if len(node.Children) == 0 {
		fmt.Printf("[执行成功] 已到达叶子节点: %s\n", node.Skill.Name())
		return results, true, nil
	}

	// 4. 深度优先遍历子节点
	for _, child := range node.Children {
		// 将当前节点的结果作为参数传给子节点
		finalResults, success, childErr := ExecuteSkillTree(ctx, child, results)

		// 如果子节点返回成功，则向上层报告成功
		if success {
			return finalResults, true, nil
		}

		// 如果子节点失败了，继续循环尝试下一个兄弟节点 (即 for 循环的下一次迭代)
		_ = childErr // 可以在这里记录日志，但不需要直接返回 err
	}

	// 5. 如果所有子节点都尝试过了都没有成功的路径
	return nil, false, fmt.Errorf("node %s 的所有分支执行均失败", node.Skill.Name())
}

func publishExecStepEvent(ctx context.Context, payload map[string]interface{}) {
	turnID, _ := runcontrol.TurnMetaFromContext(ctx)
	blackboard.GetInstance().PublishMsg(blackboard.TopicExecStepEvent, payload, turnID, 0)
}
