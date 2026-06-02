package talkAdapter

// TalkAdapter 定义表达能力的执行标准
type TalkAdapter interface {
	// Execute 接收表达内容。
	// 建议支持的类型：
	// - string: 纯文本，通常用于 TTS 合成或日志打印。
	// - []float32: 原始 PCM 音频流，通常用于直接驱动声卡。
	// - []byte: 编码后的音频数据（如 MP3/WAV 字节流）。
	Execute(data interface{}) error

	// GetType 返回适配器类型（如 "TTS", "LOG", "SPEAKER"）
	GetType() string
}
