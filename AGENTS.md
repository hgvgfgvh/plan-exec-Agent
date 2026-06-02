# Agent 能力导航（运行时）

本仓库执行类 Agent（`behaviorAgent`、`interactiveAgent`、`baseAgent`）在**每次 `CustomExecutor.Run`** 时，会由 `capabilities.BuildAgentCatalogMarkdown()` 动态生成**第一层能力目录**并注入 system prompt（非本文件的静态副本）。

## 第一层（已在上下文中）

- **MCP**：仅列出已连接的 **server 名**、功能说明、工具数、公开名前缀（`server__*`）；不枚举每个工具。
- **内置技能**：`abilities.yml` 的 Domain → Ability → Skill 树（仅已注册且启用的实例）。
- **外挂 SKILL**：`WorkSpace/skill_packs/*/SKILL.md` 的 id、title、summary。

第一层目录**不做长度截断**；MCP 逐工具 Schema 在第二层获取。

## 第二层（工具）

```
Action: get_capability_details
Action Input: {"mcp_tools":["filesystem"],"external_skills":["pack-id"],"builtin_skills":["PowerShell"]}
```

- **mcp_tools**：填 MCP **server 名**（展开该服务全部工具 Schema）或 **公开工具名**（如 `sqlite__read_query`）
- 可一次指定多个 MCP / 外挂包 / 内置技能

`capabilities.mcp.servers[].description` 可选手写第一层 MCP 功能说明；未写则按服务名或工具描述推断。

## 内置技能执行

内置技能**不能**顶格 `Action: SkillName`；须（**一次一个技能**）：

```
Action: SetExecutorStep
Action Input: {"skill":"PowerShell","args":["..."]}
```

## System Prompt 结构

1. 角色与约束说明  
2. `## 工具（Function Calling）` 统一工具表  
3. 文末 `# AGENTS.md（运行时能力目录·第一层）`（由 `FormatCatalogForExecutor` 注入；`BehaviorAgent` ↔ `behaviorAgent` 已映射）

## 相关代码

| 模块 | 职责 |
|------|------|
| `capabilities/agent_catalog.go` | 动态目录生成 |
| `capabilities/capability_details_tool.go` | 第二层详情 |
| `prefrontalCortex/customExecutor.go` | 注入 system |
| `skillpacks/apply.go` | 外挂包目录数据源 |
| `plan/delivery` | Plan 门户正文解析（与 MCP/Exec 解耦） |
| `docs/DESIGN_INTENT.md` | 设计意图（人工宪法） |

## Plan 编排与门户

- 用户主入口为 **PlanAgent**：拆 TodoList → 逐步 **BehaviorAgent** → 汇总门户回复。
- 单步交付正文经 `plan/delivery.ResolveStepDisplay`（`report.summary` 与 Exec `UserVisible` 择优），详见 `docs/DESIGN_INTENT.md`。

已移除的旧工具：`list_mcp_servers`、`list_mcp_tools`、`get_mcp_tool_schema`、`list_external_skill_packs`、`get_external_skill_document`、`skill_hierarchy_discovery`（第一层改由目录承担）。
