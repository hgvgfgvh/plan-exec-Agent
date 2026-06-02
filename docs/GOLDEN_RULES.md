# 黄金法则（来自真实事故）

## 门户不得透传 Exec 过程句

**事故来源**：用户问「有哪些 MCP」；`report_step_result.summary` 含完整表格，门户显示「我来执行本步…」。

**禁止行为**

- 门面 `StepUserFacingText` / `RecordStepOutcome` 写死优先 `UserVisible` 且取「最长 assistant 句」。

**必须行为**

- 使用 `plan/delivery.ResolveStepDisplay`；过程性 `UserVisible` 让位于实质性 `summary`。

**执行方式**

- `plan/delivery/delivery_test.go`
- `agent/runcontrol/step_report_test.go`
