package capabilities

import (
	"sync"

	"github.com/tmc/langchaingo/tools"
)

// RegisterLangchainTools 供外部包在 init 中注册「本机扩展」工具，与 MCP 工具一并按 capabilities.attach_to 挂到指定 Agent。
func RegisterLangchainTools(ts ...tools.Tool) {
	if len(ts) == 0 {
		return
	}
	pluginMu.Lock()
	defer pluginMu.Unlock()
	extraLangchainTools = append(extraLangchainTools, ts...)
}

func cloneExtraLangchainTools() []tools.Tool {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	return append([]tools.Tool(nil), extraLangchainTools...)
}

var (
	pluginMu            sync.Mutex
	extraLangchainTools []tools.Tool
)
