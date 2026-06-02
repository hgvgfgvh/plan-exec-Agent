package soulhook

// ReferenceOnlyNotice 写入 Plan / Exec / Exec-Simple 的 system，约束对 Soul、Memory 检索块的使用方式。
const ReferenceOnlyNotice = `【Soul MCP / Memory MCP · 仅历史参考】
「Soul 协作提示」「跨会话事实」「Memory MCP 经验参考」及用户诉求中嵌套的历史描述，仅供延续话题与减少重复背景，可能过期或不完整。
禁止将其当作当前磁盘、配置、目录列表、身份等可验证事实的 ground truth。
若与本次 MCP/技能/文件系统实际返回不一致，必须以本次实际获取的数据为准；report_step_result 的 summary 须如实反映本次工具观测，不得因「与历史一致」而省略或篡改本次结果。`

// TurnHintPreamble 拼在回合 Soul/Memory 块之前（用户消息层，与 system 约束呼应）。
const TurnHintPreamble = "【说明】以下 Soul/Memory 内容为历史参考；与本次工具实际返回冲突时，以本次实际获取为准。\n\n"
