package interaction

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const offlineAfterSec = 300

// Registry 维护设备在线表（Router 与 device MCP 可共享 DefaultRegistry）。
type Registry struct {
	mu      sync.RWMutex
	devices map[string]*DeviceRecord // key = channel + "/" + deviceID
}

// DefaultRegistry 进程内默认注册表。
var DefaultRegistry = NewRegistry()

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{devices: make(map[string]*DeviceRecord)}
}

func registryKey(channel, deviceID string) string {
	return channel + "/" + deviceID
}

// Touch 刷新设备心跳并标记在线。
func (r *Registry) Touch(ep Endpoint, caps ...string) {
	if r == nil {
		return
	}
	ep.Channel = trimOr(ep.Channel, ChannelWeb)
	ep.DeviceID = trimOr(ep.DeviceID, defaultDeviceID(ep.Channel))
	key := registryKey(ep.Channel, ep.DeviceID)
	now := time.Now().Unix()
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.devices[key]
	if !ok {
		rec = &DeviceRecord{ID: ep.DeviceID, Channel: ep.Channel}
		r.devices[key] = rec
	}
	rec.Online = true
	rec.LastSeen = now
	if len(caps) > 0 {
		rec.Caps = append([]string(nil), caps...)
	}
}

// Register 显式注册设备（如登录时）。
func (r *Registry) Register(rec DeviceRecord) {
	if r == nil {
		return
	}
	if rec.Channel == "" {
		rec.Channel = ChannelWeb
	}
	if rec.ID == "" {
		rec.ID = defaultDeviceID(rec.Channel)
	}
	key := registryKey(rec.Channel, rec.ID)
	if rec.LastSeen == 0 {
		rec.LastSeen = time.Now().Unix()
	}
	rec.Online = true
	r.mu.Lock()
	r.devices[key] = &rec
	r.mu.Unlock()
}

// Get 返回设备副本；不存在时 ok=false。
func (r *Registry) Get(channel, deviceID string) (DeviceRecord, bool) {
	if r == nil {
		return DeviceRecord{}, false
	}
	key := registryKey(channel, deviceID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.devices[key]
	if !ok {
		return DeviceRecord{}, false
	}
	return *rec, true
}

// ListOnline 返回在线设备（排除 source 自身可选）。
func (r *Registry) ListOnline(excludeChannel, excludeDeviceID string, limit int) []DeviceRecord {
	if r == nil {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}
	now := time.Now().Unix()
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []DeviceRecord
	for _, rec := range r.devices {
		if rec == nil {
			continue
		}
		if now-rec.LastSeen > offlineAfterSec {
			continue
		}
		if rec.Channel == excludeChannel && rec.ID == excludeDeviceID {
			continue
		}
		cp := *rec
		cp.Online = true
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// SliceForTurn 生成本回合注入用的设备切片：入站来源优先，其余在线设备一并列出。
func (r *Registry) SliceForTurn(source Endpoint, limit int) []DeviceRecord {
	if r == nil {
		return nil
	}
	source.Channel = trimOr(source.Channel, ChannelWeb)
	source.DeviceID = trimOr(source.DeviceID, defaultDeviceID(source.Channel))
	if limit <= 0 {
		limit = 8
	}
	now := time.Now().Unix()
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var out []DeviceRecord
	add := func(rec DeviceRecord) {
		if now-rec.LastSeen > offlineAfterSec {
			return
		}
		key := registryKey(rec.Channel, rec.ID)
		if seen[key] {
			return
		}
		seen[key] = true
		cp := rec
		cp.Online = true
		out = append(out, cp)
	}

	srcKey := registryKey(source.Channel, source.DeviceID)
	if rec, ok := r.devices[srcKey]; ok {
		add(*rec)
	} else {
		add(DeviceRecord{ID: source.DeviceID, Channel: source.Channel, Online: true, LastSeen: now})
	}

	var rest []DeviceRecord
	for _, rec := range r.devices {
		if rec == nil {
			continue
		}
		key := registryKey(rec.Channel, rec.ID)
		if seen[key] || now-rec.LastSeen > offlineAfterSec {
			continue
		}
		cp := *rec
		cp.Online = true
		rest = append(rest, cp)
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].Channel != rest[j].Channel {
			return rest[i].Channel < rest[j].Channel
		}
		return rest[i].ID < rest[j].ID
	})
	for _, rec := range rest {
		if len(out) >= limit {
			break
		}
		add(rec)
	}
	return out
}

func trimOr(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}
