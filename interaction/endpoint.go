package interaction

import (
	"net/url"
	"strings"
)

// EndpointFromQuery 从 HTTP 查询参数解析端点（SSE 等）。
func EndpointFromQuery(q url.Values) Endpoint {
	if q == nil {
		return Endpoint{}
	}
	return Endpoint{
		Channel:   strings.TrimSpace(q.Get("channel")),
		DeviceID:  strings.TrimSpace(q.Get("device_id")),
		SessionID: strings.TrimSpace(q.Get("session_id")),
	}
}

// TouchPresence 刷新设备在线心跳；channel 与 device_id 皆空时忽略。
func TouchPresence(ep Endpoint) {
	if strings.TrimSpace(ep.Channel) == "" && strings.TrimSpace(ep.DeviceID) == "" {
		return
	}
	DefaultRegistry.Touch(ep)
}
