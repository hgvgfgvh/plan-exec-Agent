package tts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"AgentTest/config"

	"github.com/ebitengine/oto/v3"
)

var (
	otoCtx *oto.Context
	ready  bool
)

const (
	defaultVoice = "Kiki"
	sampleRate   = 24000
	maxChars     = 400
)

func dashScopeTTSURL() string {
	return strings.TrimSpace(config.Get().Integrations.DashScope.TTSAPIURL)
}

func dashScopeAPIKey() string {
	return strings.TrimSpace(config.Get().Integrations.DashScope.APIKey)
}

func init() {
	op := &oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, readyChan, err := oto.NewContext(op)
	if err != nil {
		log.Printf("[TTS Init] 硬件驱动失败: %v\n", err)
		return
	}

	<-readyChan
	otoCtx = ctx
	ready = true
	log.Println("[TTS Init] 音频设备已就绪")
}

// Speak 现在支持通过 ctx 立即中断
func Speak(ctx context.Context, text string) error {
	if !ready {
		return fmt.Errorf("音频设备未就绪")
	}

	// 1. 检查初始状态
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	runeText := []rune(text)
	textLen := len(runeText)
	log.Printf("[TTS] 收到待播文本，字符长度: %d", textLen)

	cleanText := strings.NewReplacer(
		"#", " ", "*", " ", "-", " ", ">", " ", "---", " ", "\n", "。",
	).Replace(text)

	// 2. 分段处理逻辑
	chunks := splitText(cleanText, maxChars)
	for i, chunk := range chunks {
		// 每一段开始前检查是否已被外部取消
		select {
		case <-ctx.Done():
			log.Println("[TTS] 外部取消，停止后续分段合成")
			return ctx.Err()
		default:
		}

		log.Printf("[TTS] 正在播放分段 %d/%d...", i+1, len(chunks))
		if err := doSpeakRequest(ctx, chunk); err != nil {
			// 如果是 Context 取消导致的错误，直接退出
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[TTS] 分段播放异常: %v", err)
		}
	}

	return nil
}

func doSpeakRequest(ctx context.Context, text string) error {
	payload := map[string]interface{}{
		"model": "qwen3-tts-flash",
		"input": map[string]interface{}{
			"text":          text,
			"voice":         defaultVoice,
			"language_type": "Chinese",
		},
	}

	jsonData, _ := json.Marshal(payload)
	// 使用 NewRequestWithContext 绑定上下文，取消时会自动断开网络连接
	key := dashScopeAPIKey()
	if key == "" {
		return fmt.Errorf("DashScope API key 未配置：请在 config/app.yaml 设置 integrations.dashscope.api_key")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", dashScopeTTSURL(), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-SSE", "enable")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API 报错 (%d): %s", resp.StatusCode, string(body))
	}

	return processStream(ctx, resp.Body)
}

func processStream(ctx context.Context, body io.Reader) error {
	reader := bufio.NewReader(body)
	for {
		// 每次处理新行前检查 Context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			dataStr := strings.TrimPrefix(line, "data:")

			var result struct {
				Output struct {
					Audio struct {
						Data string `json:"data"`
					} `json:"audio"`
					FinishReason string `json:"finish_reason"`
				} `json:"output"`
				Code    string `json:"code"`
				Message string `json:"message"`
			}

			if err := json.Unmarshal([]byte(dataStr), &result); err != nil {
				continue
			}

			if result.Code != "" {
				return fmt.Errorf("SSE 错误: %s", result.Message)
			}

			if result.Output.Audio.Data != "" {
				audioData, err := base64.StdEncoding.DecodeString(result.Output.Audio.Data)
				if err == nil {
					// 将 ctx 传入播放函数
					if err := playChunk(ctx, audioData); err != nil {
						return err
					}
				}
			}

			if result.Output.FinishReason == "stop" {
				break
			}
		}
	}
	return nil
}

func playChunk(ctx context.Context, data []byte) error {
	if otoCtx == nil {
		return nil
	}
	player := otoCtx.NewPlayer(bytes.NewReader(data))
	player.Play()

	// 核心中断点：在音频播放期间轮询 Context 状态
	for player.IsPlaying() {
		select {
		case <-ctx.Done():
			// 外部一旦 cancel()，立即关闭当前 Player，声音会瞬间消失
			_ = player.Close()
			return ctx.Err()
		default:
			// 极短的休眠防止 CPU 占用过高
			time.Sleep(2 * time.Millisecond)
		}
	}
	_ = player.Close()
	return nil
}

func splitText(text string, max int) []string {
	var chunks []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= max {
			chunks = append(chunks, string(runes))
			break
		}
		splitIdx := max
		for i := max; i > 0; i-- {
			r := runes[i]
			if r == '。' || r == '！' || r == '？' || r == '；' || r == '\n' {
				splitIdx = i + 1
				break
			}
		}
		chunks = append(chunks, string(runes[:splitIdx]))
		runes = runes[splitIdx:]
	}
	return chunks
}
