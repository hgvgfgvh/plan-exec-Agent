package agent

import (
	"AgentTest/capabilities"
	"AgentTest/config"
	_func "AgentTest/func"
	"AgentTest/func/behaveFunc/attention"
	_speach "AgentTest/func/speach"
	"AgentTest/manager"
	"AgentTest/memory"
	"AgentTest/memory/dialogueHistoryArchiverManager"
	"AgentTest/prefrontalCortex"
	"context"
	"fmt"

	"github.com/tmc/langchaingo/tools"
)

// BaseAgent 基础智能体：支持长期记忆固化与多工具调度
type BaseAgent struct {
	Executor     *prefrontalCortex.CustomExecutor
	SystemPrompt string
	ModelKey     string
}

// NewBaseAgent 构造函数
func NewBaseAgent(modelKey string, ragFilePath string, recallThreshold int) (*BaseAgent, error) {
	prompt := `你是一个智能助手。

%s

【运行逻辑】：
1. 收到问题后，判断是否需要调用工具。
2. 可以按需求 同时调用多个工具
3. 注意：你只负责调用工具，绝对禁止自行编造【工具的返回结果】。
4. 只要收到工具结果（无论是否查到），你必须立即根据该结果给出自然语言总结，【严禁再次重复调用】相同的工具（如果传递进入的参数不同，可以重复调用）。
5. 注意 对无用的对话记忆进行清除

输出格式：
- 需调工具时：
Action: 工具名
Action Input: {"key":"val"}
- 回答用户时：直接说自然语言。`
	// 1. 获取模型（由 InitAgents 中 InitModelsFromConfig 注册）
	m, ok := manager.ModelManager.ModelMap[modelKey]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelKey)
	}
	attentionTool := &attention.UpdateWorkingMemoryTool{
		BasePrompt: prompt,
	}
	// 2. 初始化 RAG 处理器 (长期记忆与直觉固化)
	// recallThreshold: 同一关键词检索多少次后转为直觉
	ragProcessor := memory.NewMyRAGProcessor(recallThreshold, ragFilePath)

	archiveTool := &_func.ArchiveMemoryTool{
		Archiver: ragProcessor, // 已经有了
	}
	// 3. 组装工具列表
	toolList := []tools.Tool{
		_func.MemoryRetrieveTool{
			Searcher: ragProcessor, // 共享 RAG 指针
		},
		archiveTool,
		_speach.CreateSpeechTool(),
		attentionTool,
	}

	toolList = capabilities.AppendToolsForAgent("baseAgent", toolList)

	ex := config.Get().Executor
	agentExecutor := prefrontalCortex.NewCustomExecutor(
		"BaseAgent",
		m.(*prefrontalCortex.Mode),
		toolList,
		ex.BaseMaxSteps,
		ex.BaseMaxHistory,
		ragProcessor,
		dialogueHistoryArchiverManager.NewDialogueHistoryArchiverManager(
			m.(*prefrontalCortex.Mode), ex.DialogueArchiveTokens, ex.DialogueArchiveRounds),
	)

	archiveTool.ChatMemory = agentExecutor.Memory //TODO 注意特殊方法 需要反向将NewCustomExecutor中的记忆管理注入

	// 5. 预设系统提示词 (注入运行逻辑约束)

	ba := &BaseAgent{
		Executor:     agentExecutor,
		SystemPrompt: prompt,
		ModelKey:     modelKey,
	}

	// 让工具直接控制 ba 的 SystemPrompt
	attentionTool.TargetPromptPtr = &ba.SystemPrompt //TODO 注意特殊方法 需要反向将系统提示词注入

	return ba, nil
}

func (ba *BaseAgent) ReportActionResult(skillName string, out []interface{}, err error) {

}

func (ba *BaseAgent) Process(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 动态提取并校验 query
	var query string
	if len(args) > 0 {
		switch v := args[0].(type) {
		case string:
			query = v
		case fmt.Stringer:
			query = v.String()
		default:
			// 兜底处理：尝试转为字符串格式
			query = fmt.Sprintf("%v", v)
		}
	}

	if query == "" {
		return nil, fmt.Errorf("query is empty or invalid")
	}

	// 2. 执行核心逻辑
	// ba.Executor.Run 会处理内部的工具调用循环和记忆检索
	result, err := ba.Executor.Run(ctx, ba.SystemPrompt, query)
	if err != nil {
		return nil, fmt.Errorf("base agent execution error: %w", err)
	}

	// 3. 返回切片格式以适配插件化架构
	// 后续可以根据需要在此切片中加入 Metadata 或其他辅助信息
	return []interface{}{result}, nil
}

func (ba *BaseAgent) StartListening(ctx context.Context) {

}
