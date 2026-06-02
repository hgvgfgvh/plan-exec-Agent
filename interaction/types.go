// Package interaction 提供多设备统一入站标注、回合回执路由与设备注册表。
package interaction

import (
	"errors"
	"strings"
)

// ErrProcessTurnNotRegistered portal 未注册 ProcessTurn 时返回。
var ErrProcessTurnNotRegistered = errors.New("interaction: ProcessTurn 未注册（portal 应 init 注册）")

// 渠道常量（入站 / 回执 adapter 名）。
const (
	ChannelWeb      = "web"
	ChannelDesktop  = "desktop"
	ChannelMobile   = "mobile"
	ChannelEmbedded = "embedded"
)

// Endpoint 标识一个可回执的端点。
type Endpoint struct {
	Channel   string `json:"channel"`
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id,omitempty"`
}

// TurnRequest 统一入站请求（Web / 桌面 / 手机 / 嵌入式 bridge 共用）。
type TurnRequest struct {
	Channel   string
	DeviceID  string
	SessionID string
	Message   string
	StagingID string
	// ReplyTo 指定回执目标；零值表示回执到入站来源。
	ReplyTo Endpoint
}

// Normalize 填充默认值。
func (r *TurnRequest) Normalize() {
	r.Channel = strings.TrimSpace(r.Channel)
	if r.Channel == "" {
		r.Channel = ChannelWeb
	}
	r.DeviceID = strings.TrimSpace(r.DeviceID)
	if r.DeviceID == "" {
		r.DeviceID = defaultDeviceID(r.Channel)
	}
	r.SessionID = strings.TrimSpace(r.SessionID)
	r.Message = strings.TrimSpace(r.Message)
	r.StagingID = strings.TrimSpace(r.StagingID)
	r.ReplyTo.Channel = strings.TrimSpace(r.ReplyTo.Channel)
	r.ReplyTo.DeviceID = strings.TrimSpace(r.ReplyTo.DeviceID)
	r.ReplyTo.SessionID = strings.TrimSpace(r.ReplyTo.SessionID)
	if r.ReplyTo.Channel == "" {
		r.ReplyTo = r.SourceEndpoint()
	}
}

func defaultDeviceID(channel string) string {
	switch channel {
	case ChannelWeb:
		return "web-default"
	case ChannelDesktop:
		return "desktop-default"
	case ChannelMobile:
		return "mobile-default"
	case ChannelEmbedded:
		return "embedded-default"
	default:
		return channel + "-default"
	}
}

// SourceEndpoint 入站来源端点。
func (r *TurnRequest) SourceEndpoint() Endpoint {
	return Endpoint{
		Channel:   r.Channel,
		DeviceID:  r.DeviceID,
		SessionID: r.SessionID,
	}
}

// ReplyBinding 回合级回执绑定（Agent 主链无感，由 Deliver 消费）。
type ReplyBinding struct {
	TurnID string
	Source Endpoint
	Reply  Endpoint
}

// DeviceRecord 设备注册表条目（不全量注入 LLM）。
type DeviceRecord struct {
	ID       string
	Channel  string
	Online   bool
	LastSeen int64 // Unix 秒
	Caps     []string
}

// DeliveryPayload 回执投递载荷。
type DeliveryPayload struct {
	TurnID    string
	Event     string // outputbus: "" | delta | final | artifacts
	Source    string
	Text      string
	Artifacts []string // WorkSpace 相对路径
}
