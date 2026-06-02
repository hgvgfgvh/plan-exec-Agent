package interaction

import "testing"

func TestBindingStore(t *testing.T) {
	s := newBindingStore()
	s.Put(ReplyBinding{
		TurnID: "t-1",
		Source: Endpoint{Channel: ChannelMobile, DeviceID: "p1"},
		Reply:  Endpoint{Channel: ChannelMobile, DeviceID: "p1"},
	})
	b, ok := s.Get("t-1")
	if !ok || b.TurnID != "t-1" {
		t.Fatal("expected binding")
	}
}
