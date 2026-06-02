package capabilities

// ExecutorToolPromptSuppressor 为 true 时，CustomExecutor 不在每轮 system 的「工具清单」中列出该工具；
// 工具仍注册在 Tools 映射中，可通过 Action 使用其 Name()（例如 MCP 的公开名）。
type ExecutorToolPromptSuppressor interface {
	SuppressExecutorToolPrompt() bool
}

// ExecutorParallelUnsafe 实现该接口表示该工具的 Call 内部会写共享状态（如 Executor.Memory），
// 无法与同批次其它工具并行执行。CustomExecutor 在一个 actions 批次中只要发现一个这类工具，
// 就回退为完全串行执行该批次，以保护数据一致性。
type ExecutorParallelUnsafe interface {
	ExecutorParallelUnsafe()
}
