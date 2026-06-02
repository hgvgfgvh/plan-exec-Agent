package interaction

// ListDevices 返回在线设备列表（供 device-bridge MCP 等调用）。
func ListDevices(limit int) []DeviceRecord {
	return DefaultRegistry.ListOnline("", "", limit)
}

// DeviceStatus 查询单设备状态。
func DeviceStatus(channel, deviceID string) (DeviceRecord, bool) {
	return DefaultRegistry.Get(channel, deviceID)
}

// BindingForTurn 查询回合回执绑定（调试 / MCP）。
func BindingForTurn(turnID string) (ReplyBinding, bool) {
	return Default().Bindings().Get(turnID)
}
