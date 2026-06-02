# Harness 方法论（本仓库适用版）

防止「多轮长迭代 → 架构坍缩 / 功能回退 / 漂移」的核心思路：**意图锚点 + 可执行反馈 + 人工黄金法则**，而非仅靠对话历史或超长 prompt。

## 四层护栏

```text
1. 人工设计意图层     → DESIGN_INTENT.md（宪法，带历史；含阶段一/二铁律）
2. 架构/验收文档层     → ARCHITECTURE.md、ACCEPTANCE_RULES.md
3. 硬性自动化约束层   → go test、lintcheck、verify.Gate、artifact 校验
4. 人工巡检/黄金法则   → GOLDEN_RULES.md、ARCHITECTURE_DRIFT.md
```

## 阶段演进（AgentTest）

- **阶段一**：`plan → exec` 逐步，稳、可验收，适合复杂与无记忆命中任务。  
- **阶段二**：在阶段一之上增加 **Memory MCP + Exec-Simple**；失败必降级回阶段一；详见 `DESIGN_INTENT.md` 阶段二铁律。  
- 实现阶段二时：先冻结 Memory MCP 契约（CLCA 宪法级），再改 `planAgent` 路由，**禁止**跳过阶段一保底路径。

## Feedforward 与 Feedback（Fowler / OpenAI）

| 类型 | 时机 | 本仓库示例 |
|------|------|-----------|
| **Feedforward** | 编码前 | 读设计意图；Plan 步 `buildStepCommand` 注入路标与 tier |
| **Feedback** | 编码后 | `verify.Gate`（**当前 L1 验盘 + L2 临时关闭**）；`go test`；`report_step_result` 必填 |

**优先级（确定性优先）：**  
类型检查 / 编译 → 单元测试 → 架构测试（`lintcheck`）→ lint → CI → AI review → prompt 提醒

## 架构同步原则

- **当前代码 ≠ 正确架构**；同步时只写入已确认或已有测试覆盖的不变量。  
- 可疑差异 → `ARCHITECTURE_DRIFT.md`，不写入 `ARCHITECTURE.md`。  
- `DESIGN_INTENT.md` 只追加决策记录，不整篇覆盖。

## 黄金法则生命周期

1. 人工验收发现坍缩/回退  
2. 分析为何现有测试/文档未拦住  
3. 写入 `GOLDEN_RULES.md`（事故来源 + 禁止 + 必须 + 执行方式）  
4. **7 日内**落地为 `go test` 或 `lintcheck` 规则，否则降级为建议  

## 每次有意义修改后的坍缩自查

1. 是否改动 Plan/Exec 职责边界？  
2. 是否绕过 `sessionmemory` / `todolist` / `verify.Gate`？  
3. 是否删除或弱化已有测试？  
4. 是否引入与现有子系统竞争的重复实现？  
5. Plan 路径是否复活「用户原文关键词 → 固定链路」规则路由？  
6. 是否让 exec 跨 Plan 步保留 `ChatHistory`？  

若任一为「是」且非用户明确要求 → 修复或记入 `ARCHITECTURE_DRIFT.md`。
