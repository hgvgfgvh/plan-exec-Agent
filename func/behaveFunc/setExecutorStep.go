package behaveFunc

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/behavior/executor"
	"AgentTest/body/blackboard"
	"AgentTest/plan/skillwait"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// SetExecutorStep 异步执行单个内置技能（非树；一次仅一个 skill）。
type SetExecutorStep struct{}

func (o SetExecutorStep) Name() string {
	return "SetExecutorStep"
}

func (o SetExecutorStep) Description() string {
	return `异步执行一个内置技能。输入 JSON：{"skill":"<能力目录中的技能名>","args":[...]}。args 为传给该技能的参数列表；Schema 见 get_capability_details 的 builtin_skills。`
}

type setExecutorStepInput struct {
	Skill       string        `json:"skill"`
	Args        []interface{} `json:"args"`
	InitialArgs []interface{} `json:"initial_args,omitempty"` // 兼容旧字段名
}

func (o SetExecutorStep) Call(ctx context.Context, input string) (string, error) {
	var params setExecutorStepInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("参数格式错误: %v；需要 {\"skill\":\"...\",\"args\":[...]}", err)
	}
	skillName := strings.TrimSpace(params.Skill)
	if skillName == "" {
		return "", fmt.Errorf("缺少 skill 字段")
	}
	args := params.Args
	if len(args) == 0 && len(params.InitialArgs) > 0 {
		args = params.InitialArgs
	}

	tree, err := executor.BuildSkillTree(&executor.SkillStep{Skill: skillName})
	if err != nil {
		return "", fmt.Errorf("技能校验失败: %v", err)
	}

	turnID, hop := runcontrol.TurnMetaFromContext(ctx)
	go func() {
		execCtx, cancel := context.WithCancel(context.Background())
		execCtx = runcontrol.WithTurnMeta(execCtx, turnID, hop)
		defer cancel()

		abortCh := blackboard.GetInstance().Subscribe(blackboard.TopicAgentAbort, 1)
		go func() {
			select {
			case <-abortCh:
				cancel()
			case <-execCtx.Done():
				return
			}
		}()

		blackboard.GetInstance().PublishMsg(blackboard.TopicExecStatus, "started", turnID, hop)
		fmt.Printf("======= 执行内置技能: %s =======\n", skillName)
		finalResult, success, execErr := executor.ExecuteSkillTree(execCtx, tree, args)
		fmt.Println("========= [异步] 技能执行完毕 ==========")

		if execErr != nil {
			blackboard.GetInstance().PublishMsg(blackboard.TopicExecStatus, fmt.Sprintf("failed: %v", execErr), turnID, hop)
		} else if success {
			blackboard.GetInstance().PublishMsg(blackboard.TopicExecStatus, "completed", turnID, hop)
			skillwait.RecordResult(turnID, finalResult)
			blackboard.GetInstance().PublishMsg(blackboard.TopicExecResult, finalResult, turnID, hop)
		} else {
			blackboard.GetInstance().PublishMsg(blackboard.TopicExecStatus, "no_path_found", turnID, hop)
		}
	}()

	return fmt.Sprintf("已接收：后台异步执行内置技能 %q。请通过 exec 状态/结果观察进展。", skillName), nil
}

func CreateSetExecutorStep() tools.Tool {
	return SetExecutorStep{}
}
