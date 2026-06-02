package soulhook

import (
	"AgentTest/config"
	"fmt"
)

// SoulMCPHook Host 编排层 Soul MCP 钩子（不参与 Exec-Simple 路由）。
type SoulMCPHook struct {
	cfg      *config.App
	provider Provider
}

// NewSoulMCPHook 根据配置构造；provider 为空时按 enabled/provider 构建。
func NewSoulMCPHook(cfg *config.App, provider Provider) (*SoulMCPHook, error) {
	if cfg == nil {
		return nil, fmt.Errorf("soulhook: nil config")
	}
	if provider == nil {
		var err error
		provider, err = buildProvider(cfg)
		if err != nil {
			return nil, err
		}
	}
	return &SoulMCPHook{cfg: cfg, provider: provider}, nil
}

// ProviderName 当前插件名。
func (h *SoulMCPHook) ProviderName() string {
	if h == nil || h.provider == nil {
		return ""
	}
	return h.provider.Name()
}

// StoreEnabled 是否执行回合结束 store。
func (h *SoulMCPHook) StoreEnabled() bool {
	if h == nil || h.cfg == nil || !h.cfg.PlanSoulHook.Enabled {
		return false
	}
	if h.cfg.PlanSoulHook.StoreEnabled != nil {
		return *h.cfg.PlanSoulHook.StoreEnabled
	}
	return true
}
