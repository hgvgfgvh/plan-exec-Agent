# Interaction Router（多设备统一入站与回执）

## 职责

| 模块 | 职责 |
|------|------|
| `interaction.Router` | 入站标注、`【交互路由·本回合】` 注入（仅入站来源 + 已登记设备列表；不含回执说明）、`ReplyBinding` |
| `interaction.Registry` | 设备在线表（不全量进 LLM） |
| `interaction.Deliver` | 订阅 `outputbus`，按 `turn_id` 回执到设备（LLM 无感） |
| `portal.ProcessTurn` | Agent 主链（Plan/Behavior），由 Router 在已 `BeginTurn` 的 ctx 上调用 |
| device MCP（后续） | 主动控设备、查在线；读 `interaction.DefaultRegistry` |

## HTTP API（与 WebUI 共用）

`POST /api/chat` 扩展字段（均可选）：

```json
{
  "message": "打开嵌入式灯",
  "staging_id": "s-xxx",
  "channel": "mobile",
  "device_id": "phone-1",
  "session_id": "push-token-or-sse-id",
  "reply_to": {
    "channel": "mobile",
    "device_id": "phone-1"
  }
}
```

缺省：`channel=web`，`device_id=web-default`，回执目标 = 入站来源。

## 数据流

```
Adapter → interaction.HandleTurn → BeginTurn + Binding
       → portal.ProcessTurn(routingPrefix + planInput)
       → outputbus（带 turn_id）
       → interaction.Deliver → adapter.Push（web 为 no-op，走 SSE）
```

## 控设备 vs 回执

- **回执**：Deliver 自动推送，不写入 Plan 上下文；LLM 无感。
- **控制**：经与各设备关联的 MCP 主动控制；Plan 上下文中的「已登记设备」列表来自 Registry，具体 MCP 名由配置决定。

## 相关代码

- `interaction/` 包
- `agent/runcontrol`：`InteractionMeta`
- `agentWorkSpace/portal/gateway.go`：`RunRouterTurn` / `ProcessTurn`
- `webui/server.go`：`handleChat` 转 `interaction.Default().HandleTurn`
