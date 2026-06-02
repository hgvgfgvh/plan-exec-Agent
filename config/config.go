package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// App 为应用级配置（由 YAML 加载，零值字段在 Load 后套用默认）。
type App struct {
	// Root 为解析 paths 时的工程根目录：绝对路径，或相对「当前工作目录」。
	Root string `yaml:"root"`

	Paths struct {
		Abilities string `yaml:"abilities"`
		Memory    string `yaml:"memory"`
		// PlanMemory PlanAgent 跨轮对话长效记忆（JSONL）；空则回退 paths.memory。
		PlanMemory    string `yaml:"plan_memory"`
		Experience    string `yaml:"experience"`
		Soul          string `yaml:"soul"`
		Workspace     string `yaml:"workspace"`
		WorkspaceWord string `yaml:"workspace_word"`
		WorkspacePPT  string `yaml:"workspace_ppt"`
		// MCPBundled 为随部署打包的官方/预置 MCP 可执行文件所在根目录（相对 root）；stdio 的 command 写相对路径时与此目录配合使用。
		MCPBundled string `yaml:"mcp_bundled"`
	} `yaml:"paths"`

	Models []ModelDef `yaml:"models"`

	Agents struct {
		DefaultModel       string `yaml:"default_model"`
		RAGRecallThreshold int    `yaml:"rag_recall_threshold"`
		BehaviorZeroState  bool   `yaml:"behavior_zero_state"`
	} `yaml:"agents"`

	Executor struct {
		DialogueArchiveTokens int `yaml:"dialogue_archive_tokens"`
		DialogueArchiveRounds int `yaml:"dialogue_archive_rounds"`

		BaseMaxSteps     int `yaml:"base_max_steps"`
		BaseMaxHistory   int `yaml:"base_max_history"`
		RouterMaxSteps   int `yaml:"router_max_steps"`
		RouterMaxHistory int `yaml:"router_max_history"`

		PlanMaxSteps   int `yaml:"plan_max_steps"` // 已废弃：请用 plan_prompt_max_steps；为 0 时回退读取本字段
		PlanMaxHistory int `yaml:"plan_max_history"`
		// PlanPromptMaxSteps 写入 PlanAgent 拆步 prompt 的步骤数上限提示（默认 12）。
		PlanPromptMaxSteps int `yaml:"plan_prompt_max_steps"`
		// PlanMaxStepsPerPlan 单份 TodoList 最多保留步骤数（默认 24）。
		PlanMaxStepsPerPlan int `yaml:"plan_max_steps_per_plan"`
		// PlanMaxDispatchPerTurn 单轮 Process 最多下发步数，含重试（默认 40）。
		PlanMaxDispatchPerTurn int `yaml:"plan_max_dispatch_per_turn"`
		// PlanMaxAdjustPerStep 单步连续失败后 escalate 阈值（默认 3）。
		PlanMaxAdjustPerStep int `yaml:"plan_max_adjust_per_step"`
		// PlanResultSummaryMaxRunes 单步 report summary / 失败信息写入 TodoList 前的最大 rune（默认 2000）。
		PlanResultSummaryMaxRunes int `yaml:"plan_result_summary_max_runes"`
		// PlanStepDetailMaxRunes 单步 UserVisible 写入 result_detail 前的最大 rune（默认 24000）。
		PlanStepDetailMaxRunes int `yaml:"plan_step_detail_max_runes"`
		// PlanArchiveRounds token 超限时 DialogueHistoryArchiver 保留的最近轮数；0 则用 dialogue_archive_rounds。
		PlanArchiveRounds int `yaml:"plan_archive_rounds"`
		// PlanJSONLRAGEnabled 是否启用 Plan 本地 JSONL RAG（本能联想 + 深度检索，写入 plan_memory）。
		// 默认 false：跨会话语义记忆由 Soul/Memory MCP 承担；仍保留 ConversationBuffer 与 archiver。
		PlanJSONLRAGEnabled bool `yaml:"plan_jsonl_rag_enabled"`

		BehaviorMaxSteps   int `yaml:"behavior_max_steps"`
		BehaviorMaxHistory int `yaml:"behavior_max_history"`

		// PlanStepMaxSteps PlanAgent 单步下发时 Behavior CustomExecutor 的最大 ReAct 轮数（默认 8）。
		PlanStepMaxSteps int `yaml:"plan_step_max_steps"`

		// ExecSimpleEnabled 开启 Plan 可选的 Exec-Simple episode 快路径；无经验命中时仍走逐步 Exec。
		ExecSimpleEnabled bool `yaml:"exec_simple_enabled"`
		// ExecSimpleMaxSteps Exec-Simple 单个 episode 的最大 ReAct 轮数（默认 80）。
		ExecSimpleMaxSteps int `yaml:"exec_simple_max_steps"`
		// ExecSimpleMaxHistory Exec-Simple 保留的对话轮数，比逐步 Exec 更长（默认 240）。
		ExecSimpleMaxHistory int `yaml:"exec_simple_max_history"`
		// ExecSimpleArchiveRounds Exec-Simple 触发归档时保留的最近轮数（默认 6）。
		ExecSimpleArchiveRounds int `yaml:"exec_simple_archive_rounds"`
		// ExecSimpleMinConfidence Memory 经验命中置信度达到该值才允许 simple（默认 0.75）。
		ExecSimpleMinConfidence float64 `yaml:"exec_simple_min_confidence"`
		// ExecSimpleMaxTier 允许 simple 的最高 TodoList tier；高于该值走保守 Exec（默认 2）。
		ExecSimpleMaxTier int `yaml:"exec_simple_max_tier"`

		AffectiveMaxSteps   int `yaml:"affective_max_steps"`
		AffectiveMaxHistory int `yaml:"affective_max_history"`

		// BehaviorArchiveRounds 行为 Agent 对话归档轮数（原硬编码为 1）。
		BehaviorArchiveRounds int `yaml:"behavior_archive_rounds"`

		// RouterReflectionMaxHops 同一用户回合内 Router 反思链允许的最大跳数（Agent→Agent）。
		// Hop=0 为用户首次进入；Hop=1 为首次反思转发；Hop>=该值则 Router 丢弃反思消息，避免无界放大。
		// 默认 2。
		RouterReflectionMaxHops int `yaml:"router_reflection_max_hops"`

		// HistoryToolRoundsKeep 单次 Run 内发给模型的 history 尾部最多保留多少轮「助手回复 + 工具结果」（每轮占 2 条）；超出则丢弃更早的轮次，避免多轮 MCP/技能探测撑爆上下文。
		HistoryToolRoundsKeep int `yaml:"history_tool_rounds_keep"`
		// ToolObservationMaxRunes 单轮工具汇总文本（写入 history / Memory）的最大字符数（按 rune 计），0 表示用默认。
		ToolObservationMaxRunes int `yaml:"tool_observation_max_runes"`
		// ToolResultLineMaxRunes 并行工具结果中，单条工具返回文本的最大 rune 数，0 表示用默认。
		ToolResultLineMaxRunes int `yaml:"tool_result_line_max_runes"`
		// AIRoundMaxRunes 写入 history 的助手单条回复最大 rune（防止 reasoning 过长）；0 表示用默认。
		AIRoundMaxRunes int `yaml:"ai_round_max_runes"`

		// PromptTimeGranularitySeconds 注入 system prompt 的「当前时间」舍入粒度（秒）。
		// 0 表示用默认（300 即 5 分钟）。同一 5 分钟窗口内多次调用产生完全一致的 prompt，
		// 为未来的 LLM 响应缓存腾出命中空间；同时仍给模型粗粒度的时间感知。
		// 设为 1 即恢复秒级实时注入（缓存命中率近 0）。
		PromptTimeGranularitySeconds int `yaml:"prompt_time_granularity_seconds"`

		// DisableAPIToolCalls 为 true 时关闭 API tools/tool_calls 主路径，仅使用文本 ReAct 兜底。
		DisableAPIToolCalls bool `yaml:"disable_api_tool_calls"`

		// DisableProgressiveToolDisclosure 为 true 时不在 API tools[] 中隐藏 MCP；
		// 默认 false：有 AGENTS 地图的 Exec 仅暴露元工具，get_capability_details 后动态解锁 MCP。
		DisableProgressiveToolDisclosure bool `yaml:"disable_progressive_tool_disclosure"`

		// FacadeIntermediateAgents 列出在「工具调用循环过程中」允许将模型中间叙述推到 facadeInteraction.output
		// 给用户的 Agent。默认仅放 AffectiveInteractiveAgent（直接对话脑区），避免 Router/Behavior 内部
		// 思考链「好的我先查询…让我用 directory_tree…」泄漏给用户。
		// 名称需与 Executor.AgentName 一致（如 "AffectiveInteractiveAgent" / "BehaviorAgent" / "RouterAgent"）。
		FacadeIntermediateAgents []string `yaml:"facade_intermediate_agents"`

		// PlanDelivery Plan 编排层用户交付策略（与 Exec/MCP/渐进披露无关）。
		PlanDelivery struct {
			// DisableSingleStepSynthesize 为 true 时关闭单步交付助手归纳（默认 false=开启）。
			DisableSingleStepSynthesize bool `yaml:"disable_single_step_synthesize"`
			// SynthesizeMinRunes 触发单步归纳的最小展示文本 rune 数（0 表示 400）。
			SynthesizeMinRunes int `yaml:"synthesize_min_runes"`
			// ProgressToPortal 每步完成后向 outputbus 推送进度行（默认 false，保持仅最终一条总回复）。
			ProgressToPortal bool `yaml:"progress_to_portal"`
			// StreamSynthesizeReply 为 true 时交付助手 synthesizeUserReply 经 SSE 流式推送「计划编排」（默认 true）。
			StreamSynthesizeReply bool `yaml:"stream_synthesize_reply"`
		} `yaml:"plan_delivery"`
	} `yaml:"executor"`

	// PlanMemoryHook Plan 编排层 Memory MCP 经验插件（与 Exec 工具链解耦，见 plan/memoryhook）。
	PlanMemoryHook struct {
		// Enabled 为 true 时允许调用 Provider.Retrieve 参与 Exec-Simple 路由（仍须 executor.exec_simple_enabled）。
		Enabled bool `yaml:"enabled"`
		// Provider 插件名：noop（默认）| mcp（连接 AgentTestMemoryMCP）| 自定义 RegisterProvider 名。
		Provider string `yaml:"provider"`
		// MCPServerName 预留；当前 mcp Provider 直连独立 memory-mcp 进程。
		MCPServerName string `yaml:"mcp_server_name"`
		// MCPCommand memory-mcp 可执行文件路径（stdio）；空则跳过 mcp Provider 初始化错误由 factory 返回。
		MCPCommand string `yaml:"mcp_command"`
		// MCPEngine 传给 memory-mcp -engine：stub | test（测试内嵌已完成 TodoList）。
		MCPEngine string `yaml:"mcp_engine"`
		// MCPEnv 追加子进程环境变量（如 MEMORY_MCP_DATA_DIR）。
		MCPEnv map[string]string `yaml:"mcp_env"`
		// StoreEnabled 回合结束后是否 memory_store；nil 或未写 yaml 时视为 true；false 则仅 retrieve 路由、不写入。
		StoreEnabled *bool `yaml:"store_enabled"`
	} `yaml:"plan_memory_hook"`

	// PlanSoulHook Plan 编排层 Soul MCP 人格/议题插件（WebUI 对话材料，与 Memory 解耦，见 plan/soulhook）。
	PlanSoulHook struct {
		// Enabled 为 true 时回合前 soul_retrieve、回合后 soul_store（须 provider=mcp 且 mcp_command 有效）。
		Enabled bool `yaml:"enabled"`
		// Provider 插件名：noop（默认）| mcp（连接 AgentTestSoulMCP）。
		Provider string `yaml:"provider"`
		// MCPCommand soul-mcp 可执行文件路径（stdio）。
		MCPCommand string `yaml:"mcp_command"`
		// MCPEnv 追加子进程环境变量（如 SOUL_MCP_DATA_DIR）。
		MCPEnv map[string]string `yaml:"mcp_env"`
		// StoreEnabled 回合结束后是否 soul_store；nil 或未写 yaml 时视为 true。
		StoreEnabled *bool `yaml:"store_enabled"`
	} `yaml:"plan_soul_hook"`

	// Web 内置控制台：启用后监听 HTTP，用户通过浏览器登录并与 Router 交互；关闭时仍可使用终端 stdin。
	Web struct {
		Enabled bool   `yaml:"enabled"`
		Listen  string `yaml:"listen"`
		// Username WebUI 登录用户名（默认 admin）。
		Username string `yaml:"username"`
		// Password WebUI 登录密码（默认 ZAQ!2wsx；强烈建议在本机修改，且不要对公网暴露）。
		Password string `yaml:"password"`
	} `yaml:"web"`

	// RunView 回合运行视图：旁路监听 turnjournal 日志，异步 LLM 生成 HTML（见 runview 包）。
	// LLM 与 agents.models 解耦，便于单独配置便宜模型（OpenAI 兼容 Chat Completions）。
	RunView struct {
		Enabled        bool   `yaml:"enabled"`
		LLMAPIBase     string `yaml:"llm_api_base"`
		LLMAPIKey      string `yaml:"llm_api_key"`
		LLMModel       string `yaml:"llm_model"`
		TurnLogDir     string `yaml:"turn_log_dir"`
		OutputDir      string `yaml:"output_dir"`
		MaxBundleRunes int    `yaml:"max_bundle_runes"`
		DebounceMs     int    `yaml:"debounce_ms"`
		LLMTimeoutSec  int    `yaml:"llm_timeout_sec"`
	} `yaml:"run_view"`

	Capabilities struct {
		// AttachTo 列出要挂载「MCP 工具 + RegisterLangchainTools 注册的扩展工具」的 Agent（与 agentManager 的 map key 一致）。默认不含 routerAgent。
		AttachTo []string `yaml:"attach_to"`
		MCP      struct {
			Enabled bool           `yaml:"enabled"`
			Servers []MCPServerDef `yaml:"servers"`
		} `yaml:"mcp"`
		Security struct {
			// AuditLogPath 若非空，追加 JSONL 审计（MCP 调用 + 可选本机工具摘要）。
			AuditLogPath string `yaml:"audit_log_path"`
			// MCPMaxOutputChars 截断 MCP 工具返回文本，0 表示不截断。
			MCPMaxOutputChars int `yaml:"mcp_max_output_chars"`
			// AllowMCPServerNames 非空时仅连接列出的 server name；为空表示全部允许。
			AllowMCPServerNames []string `yaml:"allow_mcp_server_names"`
			// DenyToolNameSubstrings 若 MCP 对外工具名（含 server 前缀）包含任一串（忽略大小写）则跳过注册。
			DenyToolNameSubstrings []string `yaml:"deny_tool_name_substrings"`
		} `yaml:"security"`
		Observability struct {
			Enabled             bool `yaml:"enabled"`
			NativeToolCalls     bool `yaml:"native_tool_calls"`
			LogToolArgsMaxRunes int  `yaml:"log_tool_args_max_runes"`
			// LLMChatLogEnabled 为 true 时将每次 Chat Completions 完整请求/响应写入 LLMChatLogDir（不截断）。
			LLMChatLogEnabled bool   `yaml:"llm_chat_log_enabled"`
			LLMChatLogDir     string `yaml:"llm_chat_log_dir"`
		} `yaml:"observability"`
		// SkillPacks 扫描「目录下的子文件夹」，每个含 SKILL.md 的包为一套外部能力说明；可选 mcp.yaml / mcp.json 合并进 MCP 列表（启动期生效）。
		SkillPacks struct {
			Enabled         bool     `yaml:"enabled"`
			Roots           []string `yaml:"roots"`
			PromptMaxRunes  int      `yaml:"prompt_max_runes"`
			Watch           bool     `yaml:"watch"`             // 监视 roots 变更并热更新外挂 SKILL 地图（L1/L2）
			WatchDebounceMs int      `yaml:"watch_debounce_ms"` // 防抖毫秒，默认 1500
		} `yaml:"skill_packs"`
	} `yaml:"capabilities"`

	// Integrations 第三方 API（原 Go 源码硬编码的 Key/Endpoint 外置于此）。
	Integrations Integrations `yaml:"integrations"`

	absRoot  string   `yaml:"-"`
	Resolved resolved `yaml:"-"`
}

// Integrations 聚合各厂商集成配置。
type Integrations struct {
	DashScope      DashScopeIntegration      `yaml:"dashscope"`
	DeepSeekLegacy DeepSeekLegacyIntegration `yaml:"deepseek_legacy"`
}

// ModelDef 描述注册到 ModelManager 的模型键与驱动类型。
type ModelDef struct {
	Key    string `yaml:"key"`
	Driver string `yaml:"driver"`
}

// MCPServerDef 描述 MCP Server：stdio（command）或 streamable HTTP（endpoint）。
type MCPServerDef struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
	// Transport: stdio（默认）| http（使用官方 StreamableClientTransport）
	Transport string `yaml:"transport"`
	// stdio
	// Command：若为绝对路径则原样使用；若含路径分隔符的相对路径（如 WorkSpace/mcp_bundled/foo/foo.exe），在启动 MCP 时解析为相对应用 root 的绝对路径，便于打包分发。单文件名（如 npx、python）仍走系统 PATH。
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
	WorkDir string            `yaml:"workdir"`
	// http
	Endpoint             string            `yaml:"endpoint"`
	Headers              map[string]string `yaml:"headers"`
	HTTPMaxRetries       int               `yaml:"http_max_retries"`
	DisableStandaloneSSE bool              `yaml:"disable_standalone_sse"`
	// Description 可选：写入 AGENTS.md 第一层 MCP 服务摘要（MCP 协议本身无统一 server 说明字段）。
	Description string `yaml:"description"`
	// 治理：标签（审计用）；RatePerMinute>0 时对该 server 上全部 MCP 工具共享限流。
	Tags          []string `yaml:"tags"`
	RatePerMinute int      `yaml:"rate_per_minute"`
}

type resolved struct {
	Abilities, Memory, PlanMemory, Experience, Soul string
	Workspace, Word, PPT, MCPBundled                string
}

var (
	global *App
	mu     sync.RWMutex
)

// SetGlobal 在进程内设置当前配置（通常在 main 中调用一次）。
func SetGlobal(a *App) {
	mu.Lock()
	defer mu.Unlock()
	global = a
}

// Get 返回已加载的配置；未加载时 panic（避免静默错误路径）。
func Get() *App {
	mu.RLock()
	defer mu.RUnlock()
	if global == nil {
		panic("config: Get() called before Load/SetGlobal")
	}
	return global
}

// TryGet 在未加载时返回 nil，供极少数 init 阶段路径使用。
func TryGet() *App {
	mu.RLock()
	defer mu.RUnlock()
	return global
}

// AbsRoot 返回已解析的绝对工程根路径。
func (a *App) AbsRoot() string { return a.absRoot }

// ResolvedPaths 返回已解析的绝对资源路径。
func (a *App) ResolvedPaths() resolved { return a.Resolved }

func joinUnder(root, p string) string {
	if p == "" {
		return root
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(root, filepath.Clean(p)))
}

func applyDefaults(a *App) {
	if a.Root == "" {
		a.Root = "."
	}
	if a.Paths.Abilities == "" {
		a.Paths.Abilities = "behavior/abilities.yml"
	}
	if a.Paths.Memory == "" {
		a.Paths.Memory = "memory/my_agent_memory.jsonl"
	}
	if a.Paths.PlanMemory == "" {
		a.Paths.PlanMemory = "memory/plan_agent_memory.jsonl"
	}
	if a.Paths.Experience == "" {
		a.Paths.Experience = "experience/experience.db"
	}
	if a.Paths.Soul == "" {
		a.Paths.Soul = "agent/soul/Nexus.yml"
	}
	if a.Paths.Workspace == "" {
		a.Paths.Workspace = "WorkSpace"
	}
	if a.Paths.WorkspaceWord == "" {
		a.Paths.WorkspaceWord = "WorkSpace/word"
	}
	if a.Paths.WorkspacePPT == "" {
		a.Paths.WorkspacePPT = "WorkSpace/ppt"
	}
	if a.Paths.MCPBundled == "" {
		a.Paths.MCPBundled = "WorkSpace/mcp_bundled"
	}
	if len(a.Models) == 0 {
		a.Models = []ModelDef{
			{Key: "qwen-onnx", Driver: "qwen"},
			{Key: "deepSeek-onnx", Driver: "deepseek_onnx"},
		}
	}
	if a.Agents.DefaultModel == "" {
		a.Agents.DefaultModel = "deepSeek-onnx"
	}
	if a.Agents.RAGRecallThreshold == 0 {
		a.Agents.RAGRecallThreshold = 3
	}
	e := &a.Executor
	if e.DialogueArchiveTokens == 0 {
		e.DialogueArchiveTokens = 2500
	}
	if e.DialogueArchiveRounds == 0 {
		e.DialogueArchiveRounds = 3
	}
	if e.BaseMaxSteps == 0 {
		e.BaseMaxSteps = 4
	}
	if e.BaseMaxHistory == 0 {
		e.BaseMaxHistory = 4
	}
	if e.RouterMaxSteps == 0 {
		e.RouterMaxSteps = 12
	}
	if e.RouterMaxHistory == 0 {
		e.RouterMaxHistory = 12
	}
	if e.BehaviorMaxSteps == 0 {
		e.BehaviorMaxSteps = 40
	}
	if e.PlanStepMaxSteps == 0 {
		e.PlanStepMaxSteps = 8
	}
	if e.PlanDelivery.SynthesizeMinRunes == 0 {
		e.PlanDelivery.SynthesizeMinRunes = 400
	}
	if e.PlanMaxHistory == 0 {
		e.PlanMaxHistory = 8
	}
	if e.PlanMaxSteps == 0 {
		e.PlanMaxSteps = 8
	}
	if e.PlanPromptMaxSteps == 0 {
		if e.PlanMaxSteps > 0 {
			e.PlanPromptMaxSteps = e.PlanMaxSteps
		} else {
			e.PlanPromptMaxSteps = 12
		}
	}
	if e.PlanMaxStepsPerPlan == 0 {
		e.PlanMaxStepsPerPlan = 24
	}
	if e.PlanMaxDispatchPerTurn == 0 {
		e.PlanMaxDispatchPerTurn = 40
	}
	if e.PlanMaxAdjustPerStep == 0 {
		e.PlanMaxAdjustPerStep = 3
	}
	if e.PlanResultSummaryMaxRunes == 0 {
		e.PlanResultSummaryMaxRunes = 2000
	}
	if e.PlanStepDetailMaxRunes == 0 {
		e.PlanStepDetailMaxRunes = 24000
	}
	if e.PlanArchiveRounds == 0 {
		e.PlanArchiveRounds = e.DialogueArchiveRounds
	}
	if e.BehaviorMaxHistory == 0 {
		e.BehaviorMaxHistory = 100
	}
	if e.ExecSimpleMaxSteps == 0 {
		e.ExecSimpleMaxSteps = 80
	}
	if e.ExecSimpleMaxHistory == 0 {
		e.ExecSimpleMaxHistory = 240
	}
	if e.ExecSimpleArchiveRounds == 0 {
		e.ExecSimpleArchiveRounds = 6
	}
	if e.ExecSimpleMinConfidence == 0 {
		e.ExecSimpleMinConfidence = 0.75
	}
	if e.ExecSimpleMaxTier == 0 {
		e.ExecSimpleMaxTier = 2
	}
	if strings.TrimSpace(a.PlanMemoryHook.Provider) == "" {
		a.PlanMemoryHook.Provider = "noop"
	}
	if strings.TrimSpace(a.PlanMemoryHook.MCPEngine) == "" {
		a.PlanMemoryHook.MCPEngine = "factworld"
	}
	if strings.TrimSpace(a.PlanSoulHook.Provider) == "" {
		a.PlanSoulHook.Provider = "noop"
	}
	if e.AffectiveMaxSteps == 0 {
		e.AffectiveMaxSteps = 10
	}
	if e.AffectiveMaxHistory == 0 {
		e.AffectiveMaxHistory = 20
	}
	if e.BehaviorArchiveRounds == 0 {
		e.BehaviorArchiveRounds = 1
	}
	if e.RouterReflectionMaxHops == 0 {
		e.RouterReflectionMaxHops = 2
	}
	if e.HistoryToolRoundsKeep == 0 {
		e.HistoryToolRoundsKeep = 12
	}
	if e.ToolObservationMaxRunes == 0 {
		e.ToolObservationMaxRunes = 16000
	}
	if e.ToolResultLineMaxRunes == 0 {
		e.ToolResultLineMaxRunes = 8000
	}
	if e.AIRoundMaxRunes == 0 {
		e.AIRoundMaxRunes = 24000
	}
	if e.PromptTimeGranularitySeconds == 0 {
		e.PromptTimeGranularitySeconds = 300 // 5 分钟
	}
	if len(e.FacadeIntermediateAgents) == 0 {
		// 仅 Affective（直接对话脑区）的中间叙述允许流向用户；其它 Agent 默认沉默。
		e.FacadeIntermediateAgents = []string{"AffectiveInteractiveAgent"}
	}
	if strings.TrimSpace(a.Web.Listen) == "" {
		a.Web.Listen = ":8765"
	}
	if strings.TrimSpace(a.Web.Username) == "" {
		a.Web.Username = "admin"
	}
	if strings.TrimSpace(a.Web.Password) == "" {
		a.Web.Password = "ZAQ!2wsx"
	}
	if len(a.Capabilities.AttachTo) == 0 {
		// routerAgent 故意不挂载 MCP / RegisterLangchainTools，仅做分流；执行与外部能力在 behavior / interactive 等层。
		a.Capabilities.AttachTo = []string{
			"behaviorAgent", "execSimpleAgent", "interactiveAgent",
			"baseAgent",
		}
	}
	sec := &a.Capabilities.Security
	if sec.MCPMaxOutputChars == 0 {
		sec.MCPMaxOutputChars = 240000
	}
	obs := &a.Capabilities.Observability
	if obs.LogToolArgsMaxRunes == 0 {
		obs.LogToolArgsMaxRunes = 120
	}
	if obs.LLMChatLogEnabled && strings.TrimSpace(obs.LLMChatLogDir) == "" {
		obs.LLMChatLogDir = "WorkSpace/logs/llm_chat"
	}
	sp := &a.Capabilities.SkillPacks
	if sp.PromptMaxRunes == 0 {
		sp.PromptMaxRunes = 4000
	}
	if sp.WatchDebounceMs == 0 {
		sp.WatchDebounceMs = 1500
	}
	rv := &a.RunView
	if strings.TrimSpace(rv.OutputDir) == "" {
		rv.OutputDir = "WorkSpace/run_views"
	}
	if rv.MaxBundleRunes == 0 {
		rv.MaxBundleRunes = 12000
	}
	if rv.DebounceMs == 0 {
		rv.DebounceMs = 600
	}
	if rv.LLMTimeoutSec == 0 {
		rv.LLMTimeoutSec = 180
	}
	applyIntegrationDefaults(&a.Integrations)
}

func refreshResolved(a *App) {
	r := a.absRoot
	a.Resolved.Abilities = joinUnder(r, a.Paths.Abilities)
	a.Resolved.Memory = joinUnder(r, a.Paths.Memory)
	a.Resolved.PlanMemory = joinUnder(r, a.Paths.PlanMemory)
	a.Resolved.Experience = joinUnder(r, a.Paths.Experience)
	a.Resolved.Soul = joinUnder(r, a.Paths.Soul)
	a.Resolved.Workspace = joinUnder(r, a.Paths.Workspace)
	a.Resolved.Word = joinUnder(r, a.Paths.WorkspaceWord)
	a.Resolved.PPT = joinUnder(r, a.Paths.WorkspacePPT)
	a.Resolved.MCPBundled = joinUnder(r, a.Paths.MCPBundled)
}

// Load 读取 YAML 配置文件。相对路径均相对于 Root（默认为 "."，即当前工作目录）。
func Load(configPath string) (*App, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", configPath, err)
	}
	var a App
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	applyDefaults(&a)
	absRoot, err := filepath.Abs(a.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root %q: %w", a.Root, err)
	}
	a.absRoot = absRoot
	refreshResolved(&a)
	return &a, nil
}
