package sendTopic

import (
	"fmt"
	"regexp"
	"strings"
)

// SendTopicFacadeInteraction 把模型在「工具调用循环过程中」的中间叙述发布到 facadeInteraction.output。
//
// 注意：本函数只做「文本清洗 + 发布」，是否调用它的决定权在 customExecutor.Run 那一侧——
// 它会按 config.Executor.FacadeIntermediateAgents 白名单决定哪个 Agent 的中间思考可以流向用户。
// Behavior/Router 默认不在白名单里，避免「[BehaviorAgent] 好的我先查询…」内部独白泄漏。
//
// 清洗内容：
//   - 剥除 ReAct 风格的 Action / Action Input 区块；
//   - 剥除 <｜…｜> / <|…|> 这类模型回吐的控制符（DeepSeek/Qwen 的 buildPrompt 分隔符）；
//   - 折叠多余空行。
//
// 出口走 PublishFacadeDedup，防止短窗口内同字面消息打两次出现在用户屏幕。
func SendTopicFacadeInteraction(agentName, answer string) {
	reAction := regexp.MustCompile(`(?s)(\*\*)?Action.*?\s*\{.*?\}(\*\*)?`)

	// 针对没有 Action Input 只有单个 Action 标识的情况（兜底）
	reSingleAction := regexp.MustCompile(`(?m)^(\*\*)?Action:.*(\*\*)?$`)

	cleaned := reAction.ReplaceAllString(answer, "")
	cleaned = reSingleAction.ReplaceAllString(cleaned, "")

	cleaned = StripControlTokens(cleaned)
	cleaned = strings.ReplaceAll(cleaned, "\u00a0", " ")
	lines := strings.Split(cleaned, "\n")
	var finalLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			finalLines = append(finalLines, trimmed)
		}
	}

	finalOutput := strings.Join(finalLines, "\n\n")

	if finalOutput != "" {
		payload := finalOutput
		if n := strings.TrimSpace(agentName); n != "" {
			payload = fmt.Sprintf("[%s]\n%s", n, finalOutput)
		}
		// 中间叙述没有 turnID 上下文（agentName 已能区分来源），用空 turnID 进入去重窗口即可。
		PublishFacadeDedup("", payload)
	}
}
