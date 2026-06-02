package mouth

import (
	"AgentTest/body/mouth/talkAdapter"
	"sync"
)

// Mouth 表达器官实例：单实例单适配器模式
type Mouth struct {
	mu      sync.RWMutex
	name    string
	adapter talkAdapter.TalkAdapter // 策略接口：决定信息如何输出
}

// NewMouth 初始化嘴巴实例
func NewMouth(name string, adapter talkAdapter.TalkAdapter) *Mouth {
	return &Mouth{
		name:    name,
		adapter: adapter,
	}
}

// Speak 主动触发表达动作
// data 支持 interface{} 以兼容：
// - string (文本/命令)
// - []float32 (原始音频流)
// - []byte (编码音频数据)
func (m *Mouth) Speak(data interface{}) {
	// 使用协程异步执行，确保“表达”动作不阻塞“大脑”后续决策
	go func() {
		if m.adapter != nil {
			_ = m.adapter.Execute(data)
		}
	}()
}

// GetName 获取通道名称
func (m *Mouth) GetName() string {
	return m.name
}
