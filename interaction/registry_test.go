package interaction

import (
	"strings"
	"testing"
	"time"
)

func TestSliceForTurn_listsAllOnlineDevices(t *testing.T) {
	reg := NewRegistry()
	now := time.Now().Unix()
	reg.Register(DeviceRecord{ID: "phone-1", Channel: ChannelMobile, Online: true, LastSeen: now})
	reg.Register(DeviceRecord{ID: "pc-cat", Channel: ChannelDesktop, Online: true, LastSeen: now})

	slice := reg.SliceForTurn(Endpoint{Channel: ChannelMobile, DeviceID: "phone-1"}, 8)
	if len(slice) != 2 {
		t.Fatalf("want 2 online devices, got %d: %+v", len(slice), slice)
	}
	if slice[0].Channel != ChannelMobile || slice[0].ID != "phone-1" {
		t.Fatalf("source should be first: %+v", slice[0])
	}
	foundDesktop := false
	for _, d := range slice {
		if d.Channel == ChannelDesktop && d.ID == "pc-cat" {
			foundDesktop = true
		}
	}
	if !foundDesktop {
		t.Fatalf("missing desktop device in slice: %+v", slice)
	}
}

func TestTouchPresence_ignoredWhenEmpty(t *testing.T) {
	reg := NewRegistry()
	DefaultRegistry = reg
	TouchPresence(Endpoint{})
	if len(reg.devices) != 0 {
		t.Fatalf("expected no devices, got %d", len(reg.devices))
	}
	TouchPresence(Endpoint{Channel: ChannelWeb, DeviceID: "web-default"})
	if _, ok := reg.Get(ChannelWeb, "web-default"); !ok {
		t.Fatal("expected web device registered")
	}
}

func TestFormatRoutingBlock_multipleChannels(t *testing.T) {
	src := Endpoint{Channel: ChannelMobile, DeviceID: "phone-1"}
	devices := []DeviceRecord{
		{ID: "phone-1", Channel: ChannelMobile, Online: true},
		{ID: "pc-cat", Channel: ChannelDesktop, Online: true},
	}
	block := FormatRoutingBlock(src, devices)
	for _, sub := range []string{"mobile", "phone-1", "desktop", "pc-cat"} {
		if !strings.Contains(block, sub) {
			t.Fatalf("block missing %q: %s", sub, block)
		}
	}
}
