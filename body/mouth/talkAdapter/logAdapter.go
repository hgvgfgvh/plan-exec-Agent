package talkAdapter

import (
	"fmt"
)

type LogAdapter struct{}

func (l *LogAdapter) Execute(data interface{}) error {
	switch v := data.(type) {
	case string:
		fmt.Printf("📢 [文本输出] %s\n", v)
	case []float32:
		fmt.Printf("🎵 [音频流输出] 接收到 %d 个采样点 (PCM Float32)\n", len(v))
	case []byte:
		fmt.Printf("📦 [字节流输出] 接收到 %d bytes 数据\n", len(v))
	default:
		fmt.Printf("❓ [未知类型] 接收到无法处理的类型: %T\n", v)
	}
	return nil
}

func (l *LogAdapter) GetType() string {
	return "CONSOLE_LOG"
}
