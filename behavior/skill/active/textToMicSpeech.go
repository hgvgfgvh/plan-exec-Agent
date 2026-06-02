package active

import (
	"AgentTest/behavior/skill"
	"AgentTest/util/tts" // 假设你的 TTS 逻辑在此包中
	"context"
	"fmt"
	"log"
	"strings"
)

// TextToMicSpeech 对应 YAML 中的 Audio_Injection 领域下的技能
type TextToMicSpeech struct {
	// 可以在此处添加语音引擎配置
}

func (s *TextToMicSpeech) Name() string {
	return "Text_To_Mic_Speech"
}

func (s *TextToMicSpeech) Description() string {
	return "将文字转为语音(TTS)并通过麦克风通道实时播放"
}

func (s *TextToMicSpeech) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 检查上下文
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. 参数校验：兼容 args[0]=string 或 args[0]={"A_text":"..."}（SetExecutorStep + abilities.yml）
	text, voiceModel, err := resolveTTSTextArgs(args...)
	if err != nil {
		return nil, err
	}

	// 3. 执行核心逻辑
	log.Printf("[Skill] 触发麦克风音频注入: %s (模型: %s)", text, voiceModel)

	// 注意：为了符合 Agent 流程，通常需要考虑是“异步播放”还是“同步等待播放完成”
	// 如果需要 Agent 确认播放完了再下一步，就不加 'go' 关键字
	// 如果是背景音播放，则保留 'go'

	// 这里假设底层 tts.Speak 会处理音频路由到“虚拟麦克风”或指定输出设备
	err = tts.Speak(ctx, text)

	// 4. 组装返回结果 (对应 YAML 中的 output_schema)
	if err != nil {
		log.Printf("[Skill] TTS 播放失败: %v", err)
		return []interface{}{"fail"}, fmt.Errorf("语音播放失败: %v", err)
	}

	// 返回 status: success
	return []interface{}{"success"}, nil
}

// resolveTTSTextArgs 兼容 SetExecutorStep 两种入参：
// args[0]=string，或 args[0]={"A_text":"..."} / {"text":"..."}（与 abilities.yml 一致）。
func resolveTTSTextArgs(args ...interface{}) (text, voiceModel string, err error) {
	voiceModel = "default"
	if len(args) < 1 || args[0] == nil {
		return "", "", fmt.Errorf("Text_To_Mic_Speech 技能至少需要一个参数 'text'")
	}
	switch v := args[0].(type) {
	case string:
		text = strings.TrimSpace(v)
		if len(args) >= 2 {
			if model, ok := args[1].(string); ok && strings.TrimSpace(model) != "" {
				voiceModel = strings.TrimSpace(model)
			}
		}
	case map[string]interface{}:
		text = ttsArgString(v, "A_text", "text")
		if model := ttsArgString(v, "voice_model", "B_voice_model"); model != "" {
			voiceModel = model
		}
	default:
		return "", "", fmt.Errorf("参数 'text' 格式错误，预期为非空 string 或含 A_text 的对象")
	}
	if text == "" {
		return "", "", fmt.Errorf("参数 'text' 格式错误，预期为非空 string")
	}
	return text, voiceModel, nil
}

func ttsArgString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if raw, ok := m[k]; ok && raw != nil {
			if s, ok := raw.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func init() {
	// 注册到全局技能管理器
	skill.GlobalManager.Regist(&TextToMicSpeech{})
}
