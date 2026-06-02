---
name: agent-clca-design-zh
description: >-
  指导 Agent 编程时代的宪法级架构设计：需求分期、系统 I/O、角色与三类数据链路闭环（A）、
  设计意图与文字 MVP 互审（B）、Harness 引导器与传感器（C）、实现与回滚（D）。
  在用户进行新子系统设计、架构评审、Plan-Exec/MCP/记忆编排、或提到 CLCA、闭环契约、
  A/B/C/D 阶段、设计意图宪法、角色链路、文字 MVP 时使用。
disable-model-invocation: true
---

# Agent 时代架构设计：闭环契约法（CLCA）

## 定位

本 Skill 用于 **新子系统 / 平台能力 / 长生命周期** 的架构设计，不是每次改 bug 的全流程。

与 [ai-architecture-harness-zh](../ai-architecture-harness-zh/SKILL.md) 的关系：

| 本 Skill（CLCA） | ai-architecture-harness-zh |
|------------------|----------------------------|
| **设计前**：怎么敲定架构 | **编码中/后**：怎么防坍缩 |
| A～B：宪法与闭环 | C～D 的文档、lint、黄金法则 |
| 产出设计底稿 | 维护 `DESIGN_INTENT` 等 |

参考：[Harness Engineering（Fowler）](https://martinfowler.com/articles/harness-engineering.html)、[OpenAI Harness 工程](https://openai.com/zh-Hans-CN/index/harness-engineering/)

---

## 适用档位

| 档位 | 何时使用 | 流程 |
|------|----------|------|
| **宪法级** | 新 MCP、记忆层、多 Agent 编排、改系统 I/O 契约 | 完整 A→B→C→D |
| **功能级** | 单点增强、可快速回滚 | 迷你 A（1 页）→ D；B 可缩为 15 分钟 walkthrough |

**停手条件**：P0 清单冻结后，A/B 最多 2～3 轮互审；必须进入可运行 P1 探针，禁止无限文档。

---

## 前置：需求与系统边界（敲定后不可轻易改 P0）

### 1. 需求分期

```text
P0 — 契约与闭环必须满足；变更须重走 A/B
P1 — 首版可交付；可演进
P2 — 增强；不得破坏 P0
```

维护 `docs/requirements/P0.yaml`（及 P1/P2），每条含：`id`、`描述`、`验收点`。

### 2. 系统 I/O（封闭系统）

把系统视为 **封闭状态**；系统外一切数据与影响均视为 I/O。

必须写清：

| 项 | 内容 |
|----|------|
| 输入 | 谁传入、格式、触发时机 |
| 输出 | 谁消费、格式、失败降级 |
| 非目标 | 明确不做什么 |

**跨进程 / MCP 额外必填**（避免 Host 与执行 Agent 混层）：

| 项 | 内容 |
|----|------|
| 调用方 | Host 钩子 vs 执行 Agent `tool_calls`（二选一或写清分工） |
| 载荷形态 | 是否 **仅 string**；Host 专有结构体不得 leak 为 MCP required 字段 |
| 失败策略 | 超时/错误时 Host 是否必须降级（如 retrieve 空 hints 不阻断主流程） |

---

## A 阶段：角色 + 三类链路 + 闭环

### A.1 划分角色

角色 = **逻辑职责**，不是组织职务。每个角色需：

- 功能边界（做什么 / 不做什么）
- 与 P0 需求的映射 ID
- 能否自闭环一段子链路

常见角色类型（按需取用，勿堆砌）：Host/编排、Plan、Exec、Exec-Simple、Memory 服务、Memory Agent（进程内）、主 Agent、传感器（lint/测试）。

### A.2 三类数据链路

| 类型 | 含义 | 例子 |
|------|------|------|
| **业务流** | 实现功能的最小数据流 | user_message → retrieve → inject → Process |
| **优化流** | 提升系统能力 | episode 完成 → memory_store；路由 exec-simple |
| **反馈流** | 错误/事故倒回系统 | retrieve 超时 → 空 hints；simple 失败 → pitfall 入库 |

每条链路标明：**方向**（A→B、双向、仅订阅）、**载荷**、**触发条件**。

建议落成三张简图（可 ASCII/Mermaid）：业务序列图、优化流标注、反馈流虚线。

### A.3 闭环验收（跳出山外）

检查表 — 全部满足才进入 B：

- [ ] 每个 P0 需求至少一条业务流覆盖
- [ ] 无 **孤立角色**（无需求、无链路映射的删掉）
- [ ] 无 **孤立链路**（无来源/去向/触发说明的删掉）
- [ ] 优化流/反馈流有明确触发，非「也许以后有用」
- [ ] 失败与降级路径在反馈流或业务流中可追踪
- [ ] **至少一条失败路径** 能从输入追到可观测输出（非仅 happy path）

产出物模板见 [templates.md](templates.md) 的「A 阶段底稿」。

**不通过 → 回滚 A，不进入 B。**

---

## B 阶段：设计意图宪法 + AI 分析 + 文字 MVP

### B.1 设计意图宪法

人工维护 `docs/DESIGN_INTENT.md`：

- **只追加**历史决策（日期 + 标题），禁止覆盖旧条目
- 记录「为什么」而不只是「是什么」
- 含：非目标、P0 契约、关键取舍
- B 阶段末尾追加 **开放问题**（3～10 条），避免 D 阶段 Agent 自行发明默认值

Agent 可协助起草，**人批准**后写入。

### B.2 AI 架构分析

将以下一并交给 AI 审查：

1. 背景与 P0/P1/P2
2. 系统 I/O（含 MCP/Host 分工行）
3. A 阶段底稿（角色+链路+需求映射）
4. 设计意图草案

审查维度：

- [ ] 隐患（竞态、一致性、安全、成本）
- [ ] 可演化（P1/P2 预留，非过度设计）
- [ ] 优化/反馈调节预留点
- [ ] 通用性（相似需求能否微调承接）
- [ ] **便于 Agent 读 repo 实现**（命名、边界、可测试）
- [ ] **跨边界契约**（MCP 是否 string-only；执行 Agent 是否误挂载外部能力）

### B.3 文字 MVP 互审

让 AI **扮演「系统已上线」**，按设计意图模拟完整业务交互：

- 用户输入（操作/需求）
- 系统输出 + **标明经过的角色与链路**（可附简图）
- 覆盖：happy path、失败回退、边界（寒暄/超时/无权限）

人机用 A/B 校验点 **互怼**：

- 认同 → 进入 C
- 歧义无法取舍 → **回滚 A**

固定产出：一条 happy path 序列图 + 一条失败回退序列图 + 需求映射表（见 [templates.md](templates.md)）。

---

## C 阶段：Harness 化（引导器 + 传感器）

### C.1 文档分层

| 文档 | 维护者 | 说明 |
|------|--------|------|
| `DESIGN_INTENT.md` | **人** | 宪法，历史追加 |
| `ARCHITECTURE.md` | Agent 阶段性同步 | 服从宪法 |
| `ACCEPTANCE_RULES.md` | Agent 阶段性同步 | 可执行验收 |
| `GOLDEN_RULES.md` | 人+事故 | 强规则，防坍缩 |
| `ARCHITECTURE_DRIFT.md` | Agent+人 | 意图 vs 实现差距 |

**同步时机**（不要求每 commit 全量更新）：P1 探针通过、里程碑 PR 合并前、或 DESIGN_INTENT 变更后 — 触发 ARCHITECTURE / ACCEPTANCE 对比同步。

### C.2 引导器 vs 传感器

| 类型 | 性质 | 载体 |
|------|------|------|
| **引导器（软）** | 语义、引导 | AGENTS.md、架构说明、prompt 契约 |
| **传感器（硬）** | 确定、快速 | lint、类型检查、单元/集成测试 |

原则：**长迭代中软约束会漂移，硬约束不可省。**

计算型（测试/lint）尽量 **左移**；昂贵检查（大范围审查、变异测试）放集成后。

### C.3 架构基石测试

工作流：

1. 从 `DESIGN_INTENT` / ARCHITECTURE 标出 **P0 基石节点**（如「每 user 消息触发 retrieve」）
2. Agent 对照代码找 **可观测规律**（调用了谁、写了什么日志、什么状态迁移）
3. 为每个基石写 **少量** 针对性测试（每基石 1～3 个），命名含需求 ID

例：`TestP0_MemoryRetrieve_OnUserMessage`

### C.4 黄金法则

定期人工巡检；发现 Agent 在引导+传感器下仍犯的错 → 追加 `GOLDEN_RULES.md`（来源、禁止、正确做法、能否自动化）。

### C.5 偏离分流（勿事事回滚 A）

| 偏离类型 | 处理 |
|----------|------|
| P0 契约/系统 I/O 错了 | 回滚 **A** |
| 实现 bug、测试失败 | **C** 修代码 + 传感器 |
| 实现超前但未违约 | 记 **ARCHITECTURE_DRIFT**，人裁定 |

---

## D 阶段：实现与回滚

- 按 P1 实现；**P1 必须包含可运行 smoke/探针**
- 架构级偏离 P0 → **回滚 A**，不是堆 prompt 修补
- 功能级偏离 → 修实现或迷你 A

---

## Agent 执行本 Skill 时的行为

1. **先问档位**：宪法级还是功能级？P0 是否已列出？
2. **A 阶段**：输出角色表、链路表、需求映射表、孤立性审查结果；有孤立则标红并建议删除/合并
3. **B 阶段**：起草 DESIGN_INTENT 片段 + 风险清单 + 文字 MVP 两轮交互脚本；不跳过互审问题
4. **C 阶段**：提议基石列表与测试名；不替代人写宪法
5. **禁止**：未冻结 P0 就大量写实现；用实现便利修改 P0 契约；删除 DESIGN_INTENT 历史条目

---

## 快速检查（宪法级开工前）

```text
- [ ] P0/P1/P2 已写（含 docs/requirements/P0.yaml）
- [ ] 系统 I/O + 非目标 + MCP/Host 分工 已写
- [ ] A：角色-链路-需求闭环表完成且无孤立
- [ ] A：至少一条失败路径可追踪
- [ ] B：DESIGN_INTENT 草案 + 开放问题 + 文字 MVP 双方认可
- [ ] C：lint/基石测试计划已列
- [ ] P1 smoke 探针已定义
```

---

## 本仓库示例（AgentTest 生态）

| 子系统 | 档位 | 宪法位置 |
|--------|------|----------|
| Plan 门户 / delivery | 功能级～宪法级 | `docs/DESIGN_INTENT.md` |
| Memory MCP 第三层 | 宪法级 | `AgentTestMemoryMCP/docs/DESIGN_INTENT.md` |

新子系统优先在**该子系统 repo** 维护 `docs/DESIGN_INTENT.md`，再在 Host（AgentTest）记集成 P0。

---

## 延伸阅读

- 产出模板：[templates.md](templates.md)
- 编码期护栏：[ai-architecture-harness-zh](../ai-architecture-harness-zh/SKILL.md)
