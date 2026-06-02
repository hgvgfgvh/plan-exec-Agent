package memoryhook

import (
	"AgentTest/config"
	"fmt"
	"sync"
)

var (
	defaultMu   sync.RWMutex
	defaultHook *MemoryMCPHook
)

// InitFromConfig 按 app.yaml 的 plan_memory_hook 初始化全局钩子（在 InitAgents 前调用）。
func InitFromConfig(cfg *config.App) error {
	hook, err := NewMemoryMCPHook(cfg, nil)
	if err != nil {
		return err
	}
	SetDefault(hook)
	return nil
}

// SetDefault 替换全局 MemoryMCPHook（测试或 Host 注入自定义 Provider 后调用）。
func SetDefault(h *MemoryMCPHook) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultHook = h
}

// Default 返回全局钩子；未初始化时返回仅 noop 的保守实例。
func Default() *MemoryMCPHook {
	defaultMu.RLock()
	h := defaultHook
	defaultMu.RUnlock()
	if h != nil {
		return h
	}
	cfg := config.TryGet()
	if cfg == nil {
		return &MemoryMCPHook{provider: NoopProvider{}}
	}
	fallback, err := NewMemoryMCPHook(cfg, NoopProvider{})
	if err != nil {
		return &MemoryMCPHook{cfg: cfg, provider: NoopProvider{}}
	}
	return fallback
}

// MustDefault 同 Default，但未初始化时 panic（仅测试用）。
func MustDefault() *MemoryMCPHook {
	h := Default()
	if h == nil {
		panic(fmt.Errorf("memoryhook: default hook unavailable"))
	}
	return h
}
