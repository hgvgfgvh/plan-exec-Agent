package interaction

import (
	"strings"
	"testing"
)

func TestTurnRequestNormalize(t *testing.T) {
	req := TurnRequest{Message: "hi"}
	req.Normalize()
	if req.Channel != ChannelWeb {
		t.Fatalf("channel %q", req.Channel)
	}
	if req.ReplyTo.Channel != ChannelWeb {
		t.Fatalf("reply channel %q", req.ReplyTo.Channel)
	}
}

func TestFormatRoutingBlock(t *testing.T) {
	src := Endpoint{Channel: ChannelMobile, DeviceID: "phone-1"}
	devices := []DeviceRecord{
		{ID: "phone-1", Channel: ChannelMobile, Online: true},
		{ID: "board-2", Channel: ChannelEmbedded, Online: true},
	}
	block := FormatRoutingBlock(src, devices)
	for _, sub := range []string{
		"mobile", "phone-1", "embedded", "board-2",
		"已登记设备", "经与设备对应关联的 MCP 主动控制",
	} {
		if !strings.Contains(block, sub) {
			t.Fatalf("block missing %q: %s", sub, block)
		}
	}
	for _, absent := range []string{"回执", "device-bridge", "勿调用推送类 MCP"} {
		if strings.Contains(block, absent) {
			t.Fatalf("block should not contain %q: %s", absent, block)
		}
	}
}

func TestPrefixPlanInput_separatesRoutingFromUser(t *testing.T) {
	got := PrefixPlanInput("【交互路由·本回合】\n- 入站来源: web / device_id=web-default\n\n", "你好")
	if !strings.Contains(got, "web-default\n\n你好") {
		t.Fatalf("expected blank line before user text: %q", got)
	}
}
