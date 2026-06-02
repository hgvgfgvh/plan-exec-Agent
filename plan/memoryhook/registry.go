package memoryhook

import (
	"AgentTest/config"
	"fmt"
	"strings"
	"sync"
)

type providerFactory func(cfg *config.App) (Provider, error)

var (
	providerMu        sync.RWMutex
	providerFactories = map[string]providerFactory{
		"noop": func(*config.App) (Provider, error) { return NoopProvider{}, nil },
	}
)

// RegisterProvider 注册可配置的经验 Provider（插件名与 config.plan_memory_hook.provider 一致）。
func RegisterProvider(name string, factory providerFactory) {
	providerMu.Lock()
	defer providerMu.Unlock()
	providerFactories[strings.ToLower(strings.TrimSpace(name))] = factory
}

func buildProvider(cfg *config.App) (Provider, error) {
	if cfg == nil {
		return NoopProvider{}, nil
	}
	name := strings.ToLower(strings.TrimSpace(cfg.PlanMemoryHook.Provider))
	if name == "" {
		name = "noop"
	}
	providerMu.RLock()
	factory, ok := providerFactories[name]
	providerMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("memoryhook: unknown provider %q", name)
	}
	return factory(cfg)
}
