# plan/memoryhook

Plan 层 **Memory MCP 经验插件**（与 `PlanAgent`、Exec 工具链解耦）。

## 配置（`config/app.yaml`）

```yaml
executor:
  exec_simple_enabled: true   # 快路径总开关

plan_memory_hook:
  enabled: true
  provider: mcp
  mcp_command: "C:/DATA/GODATA/AgentTestMemoryMCP/memory-mcp.exe"
  mcp_engine: test
  mcp_env:
    MEMORY_MCP_DATA_DIR: "C:/DATA/GODATA/AgentTestMemoryMCP/data"
```

`provider: noop` 时不连接 MCP，路由恒为保守 Exec。

记忆 **3D 拓扑控制台** 由 Memory MCP 进程在 stdio 模式下**自行**拉起（默认 http://127.0.0.1:8091/console/），见 `AgentTestMemoryMCP` README；Host 无需额外配置。

## 内置 MCP Provider（`provider: mcp`）

`mcp_provider.go` 在 `init()` 注册；stdio 启动 `AgentTestMemoryMCP`，调用 `memory_retrieve` / `memory_store`（字符串 JSON，无固定业务 Schema）。

**factworld**（默认 `-engine factworld`）：store 后异步规则抽取写入 `facts.jsonl`；retrieve 按 context 打分命中真实事实，hints 含 `---memory-route---` JSON 块。

**test**（`-engine test`）：内嵌 fixture，仅用于 CI/回归。

构建 MCP：

```powershell
cd C:\DATA\GODATA\AgentTestMemoryMCP
go build -o memory-mcp.exe ./cmd/memory-mcp
```

## 自定义 Provider

在 `InitAgents` / `memoryhook.InitFromConfig` **之前**注册其它名：

```go
memoryhook.RegisterProvider("my", func(cfg *config.App) (memoryhook.Provider, error) {
    return myProvider{}, nil
})
```

```go
type Provider interface {
    Name() string
    Retrieve(ctx context.Context, req memoryhook.RetrieveRequest) (memoryhook.Experience, error)
}
```

## PlanAgent 集成点

`planAgent.Process` 仅在拆步后调用一行：

```go
route := memoryhook.Default().DecideRoute(ctx, memoryhook.RouteInput{...})
```

路由护栏（tier、confidence、开关）均在 `memory_mcp_hook.go`，勿写回 `planAgent.go`。

## Store（OnTurnStore）

`portal.RunRouterTurn` 在 `Process` 返回后调用 `memoryhook.StoreTurnAfterProcess`（异步 `memory_store`，失败不阻断门户）。

| 开关 | 效果 |
|------|------|
| `plan_memory_hook.enabled: false` | 不 retrieve 路由、不 store |
| `provider: noop` | 不连 MCP，store 无 Storer，静默跳过 |
| `store_enabled: false` | 仍可做 retrieve 路由，回合结束不写入 |

关闭方式：整段 `plan_memory_hook.enabled: false`，或仅 `store_enabled: false`，或 `provider: noop`。**不修改** Plan/Behavior/Executor 主路径。
