---
name: ai-architecture-harness-zh
description: >-
  Agent 编码期架构护栏（中文版入口）：防止架构坍缩、功能回退与长程漂移。
  维护 DESIGN_INTENT、ARCHITECTURE、验收规则、黄金法则；lint 与基石测试左移。
  在编码中/后、同步架构文档、添加黄金法则、架构测试、或提到架构护栏、传感器、引导器时使用。
  新子系统设计请先读 agent-clca-design-zh。
---

# AI 编码架构护栏（Harness · 中文入口）

## 与 CLCA 的分工

| 阶段 | Skill |
|------|--------|
| 设计前 A～B | [agent-clca-design-zh](../agent-clca-design-zh/SKILL.md) |
| 编码中/后 C～D | **本 Skill**（正文见英文完整版） |

## AgentTest 宪法路径

本仓库优先读：

`Agent编码防止架构坍塌的处理方法论/`

- `DESIGN_INTENT.md` — **阶段一 + 阶段二铁律**（含 Memory MCP、Exec-Simple、F2-1～F2-9）
- `ARCHITECTURE.md` — §1～13 现网；§14 阶段二目标
- `ACCEPTANCE_RULES.md` — §I 阶段二规划验收
- `ARCHITECTURE_DRIFT.md` — DI-9～DI-13 未实现项

阶段二设计前可先用 [agent-clca-design-zh](../agent-clca-design-zh/SKILL.md) 做 A/B 闭环。

## 执行顺序

1. 非平凡代码修改前：读 `DESIGN_INTENT.md`（宪法，人维护、只追加历史）
2. 读 `ARCHITECTURE.md`、`ACCEPTANCE_RULES.md`、`GOLDEN_RULES.md`（若有）
3. 小步修改；用项目最强自动化验证（如 `go test ./lintcheck/...`、`go test ./...`）

## 四层护栏模型

```text
1. 人 — 设计意图（DESIGN_INTENT）
2. Agent 同步 — ARCHITECTURE / ACCEPTANCE_RULES
3. 硬约束 — lint、类型检查、基石单测
4. 人 — 黄金法则（事故驱动）
```

## 引导器 vs 传感器

| 类型 | 载体 |
|------|------|
| 引导器（软） | AGENTS.md、架构说明、prompt |
| 传感器（硬） | lint、测试；**长迭代不可省** |

计算型检查 **左移**；昂贵检查放集成后。

## 文档推荐布局

```text
AGENTS.md                      # 运行时能力目录，非宪法
docs/DESIGN_INTENT.md          # 人维护，历史追加
docs/ARCHITECTURE.md
docs/ACCEPTANCE_RULES.md
docs/GOLDEN_RULES.md
docs/ARCHITECTURE_DRIFT.md
docs/requirements/P0.yaml      # CLCA 前置
```

## 本仓库（AgentTest）

| 文件 | 角色 |
|------|------|
| `docs/DESIGN_INTENT.md` 或 `Agent编码防止架构坍塌的处理方法论/` | 宪法 |
| 根 `AGENTS.md` | **仅**运行时能力目录 |

子系统（如 `AgentTestMemoryMCP`）在**各自 repo** 维护 `docs/DESIGN_INTENT.md`。

## 架构同步（里程碑触发，非每 commit）

1. 读 DESIGN_INTENT
2. 对照代码，差距写入 ARCHITECTURE_DRIFT（分类：aligned / debt / violates / needs decision）
3. 已确认部分写入 ARCHITECTURE.md

**禁止**用当前实现自动覆盖宪法。

## 基石测试工作流

1. 从宪法标 P0 基石节点
2. 从代码找可观测调用链/状态迁移
3. 每基石 1～3 个测试，命名含需求 ID（`TestP0_...`）

## 黄金法则

事故 → GOLDEN_RULES.md（来源、禁止、正确做法、能否自动化）→ 尽量补传感器。

## 坍缩自检（改完代码前）

- 是否改坏分层/绕过核心抽象？
- 是否削弱测试？
- 是否引入重复子系统？
- 风险记入 DRIFT 或 GOLDEN_RULES

## 完整英文细则

与仓库内实现细节、示例格式同步时，对照：

- [../ai-architecture-harness/SKILL.md](../ai-architecture-harness/SKILL.md)

## 参考

- [Harness Engineering（Fowler）](https://martinfowler.com/articles/harness-engineering.html)
- [OpenAI Harness 工程](https://openai.com/zh-Hans-CN/index/harness-engineering/)
