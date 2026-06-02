package manager

import (
	"AgentTest/config"
	"AgentTest/entity"
	"AgentTest/prefrontalCortex"
	"fmt"
	"strings"
	"sync"
)

type modelManager struct {
	ModelMap map[string]entity.ModeInterface
}

var ModelManager modelManager

var modelInitOnce sync.Once

// InitModelsFromConfig 根据配置注册所有模型（仅执行一次）。
func InitModelsFromConfig(cfg *config.App) {
	modelInitOnce.Do(func() {
		ModelManager.ModelMap = make(map[string]entity.ModeInterface)
		for _, m := range cfg.Models {
			if m.Key == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(m.Driver)) {
			case "qwen", "qwen-onnx":
				ModelManager.ModelMap[m.Key] = prefrontalCortex.NewMode(m.Key, prefrontalCortex.NewQwenModel())
			case "deepseek_onnx", "deepseek-onnx", "onnx-qwen", "onnx_qwen":
				ModelManager.ModelMap[m.Key] = prefrontalCortex.NewMode(m.Key, prefrontalCortex.NewONNXModelQwen())
			default:
				fmt.Printf("model manager: 未知 driver %q，跳过模型 key=%q\n", m.Driver, m.Key)
			}
		}
	})
}
