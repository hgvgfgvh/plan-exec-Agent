package interaction

import (
	"fmt"
	"log"
)

// DeliverAdapter 将 outputbus / 产物回执投递到具体渠道。
type DeliverAdapter interface {
	Channel() string
	Push(sessionID string, payload DeliveryPayload) error
}

type webAdapter struct{}

func (webAdapter) Channel() string { return ChannelWeb }
func (webAdapter) Push(_ string, _ DeliveryPayload) error {
	// Web 仍由 GET /api/events 订阅全量 outputbus；避免双推。
	return nil
}

type loggingAdapter struct {
	channel string
}

func (a loggingAdapter) Channel() string { return a.channel }
func (a loggingAdapter) Push(sessionID string, p DeliveryPayload) error {
	log.Printf("[interaction/deliver] channel=%s session=%s turn=%s event=%q source=%s text_len=%d artifacts=%d",
		a.channel, sessionID, p.TurnID, p.Event, p.Source, len(p.Text), len(p.Artifacts))
	return nil
}

func defaultAdapters() []DeliverAdapter {
	return []DeliverAdapter{
		webAdapter{},
		loggingAdapter{channel: ChannelDesktop},
		loggingAdapter{channel: ChannelMobile},
		loggingAdapter{channel: ChannelEmbedded},
	}
}

// AdapterFor 按渠道查找 adapter。
func AdapterFor(adapters []DeliverAdapter, channel string) DeliverAdapter {
	for _, a := range adapters {
		if a != nil && a.Channel() == channel {
			return a
		}
	}
	return loggingAdapter{channel: channel}
}

// PushToEndpoint 向指定端点投递。
func PushToEndpoint(adapters []DeliverAdapter, ep Endpoint, payload DeliveryPayload) error {
	a := AdapterFor(adapters, ep.Channel)
	if a == nil {
		return fmt.Errorf("no adapter for channel %q", ep.Channel)
	}
	return a.Push(ep.SessionID, payload)
}
