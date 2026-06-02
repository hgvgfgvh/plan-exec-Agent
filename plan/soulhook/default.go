package soulhook

import (
	"AgentTest/config"
	"fmt"
	"sync"
)

var (
	defaultMu   sync.RWMutex
	defaultHook *SoulMCPHook
)

// InitFromConfig 按 app.yaml 的 plan_soul_hook 初始化全局钩子。
func InitFromConfig(cfg *config.App) error {
	hook, err := NewSoulMCPHook(cfg, nil)
	if err != nil {
		return err
	}
	SetDefault(hook)
	return nil
}

// SetDefault 替换全局钩子（测试用）。
func SetDefault(h *SoulMCPHook) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultHook = h
}

// Default 返回全局钩子；未初始化时返回 noop 保守实例。
func Default() *SoulMCPHook {
	defaultMu.RLock()
	h := defaultHook
	defaultMu.RUnlock()
	if h != nil {
		return h
	}
	cfg := config.TryGet()
	if cfg == nil {
		return &SoulMCPHook{provider: NoopProvider{}}
	}
	fallback, err := NewSoulMCPHook(cfg, NoopProvider{})
	if err != nil {
		return &SoulMCPHook{cfg: cfg, provider: NoopProvider{}}
	}
	return fallback
}

// MustDefault 同 Default，未初始化时 panic（测试用）。
func MustDefault() *SoulMCPHook {
	h := Default()
	if h == nil {
		panic(fmt.Errorf("soulhook: default hook unavailable"))
	}
	return h
}
