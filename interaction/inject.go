package interaction

import (
	"fmt"
	"strings"
)

// FormatRoutingBlock 生成注入 Plan 输入的路由说明（仅入站来源与已登记设备；回执由 Deliver 处理，不写入 Plan 上下文）。
func FormatRoutingBlock(source Endpoint, devices []DeviceRecord) string {
	source.Channel = trimOr(source.Channel, ChannelWeb)
	var b strings.Builder
	b.WriteString("【交互路由·本回合】\n")
	b.WriteString(fmt.Sprintf("- 入站来源: %s / device_id=%s\n", source.Channel, source.DeviceID))
	b.WriteString("- 已登记设备（经与设备对应关联的 MCP 主动控制）:\n")
	if len(devices) == 0 {
		b.WriteString("  （当前无已登记设备）\n")
	} else {
		for _, d := range devices {
			line := fmt.Sprintf("  - %s / %s", d.Channel, d.ID)
			if len(d.Caps) > 0 {
				line += " caps=" + strings.Join(d.Caps, ",")
			}
			if !d.Online {
				line += " (offline)"
			}
			b.WriteString(line + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n\n"
}

// PrefixPlanInput 将路由块置于用户输入之前。
func PrefixPlanInput(routingBlock, planInput string) string {
	routingBlock = strings.TrimSpace(routingBlock)
	planInput = strings.TrimSpace(planInput)
	if routingBlock == "" {
		return planInput
	}
	if planInput == "" {
		return routingBlock
	}
	return routingBlock + "\n\n" + planInput
}
