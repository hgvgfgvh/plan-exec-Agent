package speach

import (
	"AgentTest/util/tts"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	// 假设你的 tts 包路径如下，或者直接写在同一个 main 包里

	"github.com/tmc/langchaingo/tools"
)

// SpeechTool 实现 tools.Tool 接口
type SpeechTool struct {
	mu sync.Mutex // 2. 增加互斥锁
}

// Name 工具名称
func (s SpeechTool) Name() string {
	return "speech"
}

// Description 描述，引导 Agent 何时使用该工具
func (s SpeechTool) Description() string {
	return `主动积极调用！使用扬声器向用户进行音频交互。输入应为一个包含 "content" 的 JSON 字符串，和设备（目前可以使用的设备：PC、Robot1），例如：{"content": "你好，这是测试内容","way": "PC"}`
}

// Call 执行逻辑：调用我们之前写好的 tts.Speak 方法
func (s SpeechTool) Call(ctx context.Context, input string) (string, error) {
	s.mu.Lock()         // 3. 进入方法立即加锁
	defer s.mu.Unlock() // 执行结束自动解锁
	var params struct {
		Content string `json:"content"`
		Way     string `json:"way"`
	}

	// 解析 Agent 传来的参数
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		// 简单的容错处理
		if input != "" {
			params.Content = input
		} else {
			return "", fmt.Errorf("speech_by_pc 输入参数解析失败: %v", err)
		}
	}

	// 调用底层 tts 包的 Speak 方法
	log.Printf("[Agent Tool] 正在播放音频: %s", params.Content)

	if params.Way == "PC" {
		tts.Speak(ctx, params.Content)
	}

	if err != nil {
		return "", fmt.Errorf("语音合成执行失败: %v", err)
	}

	return "语音播放成功", nil
}

// CreateSpeechTool 构造函数
func CreateSpeechTool() tools.Tool {
	return SpeechTool{}
}
