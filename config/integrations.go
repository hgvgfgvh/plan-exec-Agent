package config

import "strings"

// DashScopeIntegration 阿里云 DashScope（通义）：Chat / Embedding / TTS / 视觉等。
type DashScopeIntegration struct {
	APIKey string `yaml:"api_key"`
	// OpenAICompatibleBaseURL 供 openai-go SDK（Embedding、视觉）使用。
	OpenAICompatibleBaseURL string `yaml:"openai_compatible_base_url"`
	// ChatCompletionsEndpoint 供 QwenModel、联网搜索等直连 HTTP 使用。
	ChatCompletionsEndpoint string `yaml:"chat_completions_endpoint"`
	ChatModel               string `yaml:"chat_model"`
	EmbeddingModel          string `yaml:"embedding_model"`
	TTSAPIURL               string `yaml:"tts_api_url"`
	VisionModel             string `yaml:"vision_model"`
	VisionModelSimple       string `yaml:"vision_model_simple"`
}

// DeepSeekLegacyIntegration 主链 deepseek_onnx 驱动（与 plan_memory/soul 等 MCP env 独立）。
type DeepSeekLegacyIntegration struct {
	ChatCompletionsEndpoint string `yaml:"chat_completions_endpoint"`
	APIKey                  string `yaml:"api_key"`
	Model                   string `yaml:"model"`
	Debug                   bool   `yaml:"debug"`
}

func applyIntegrationDefaults(i *Integrations) {
	ds := &i.DashScope
	if strings.TrimSpace(ds.OpenAICompatibleBaseURL) == "" {
		ds.OpenAICompatibleBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if strings.TrimSpace(ds.ChatCompletionsEndpoint) == "" {
		ds.ChatCompletionsEndpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	}
	if strings.TrimSpace(ds.ChatModel) == "" {
		ds.ChatModel = "qwen3-max"
	}
	if strings.TrimSpace(ds.EmbeddingModel) == "" {
		ds.EmbeddingModel = "text-embedding-v3"
	}
	if strings.TrimSpace(ds.TTSAPIURL) == "" {
		ds.TTSAPIURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	}
	if strings.TrimSpace(ds.VisionModel) == "" {
		ds.VisionModel = "qwen3.5-plus"
	}
	if strings.TrimSpace(ds.VisionModelSimple) == "" {
		ds.VisionModelSimple = "qwen-vl-plus"
	}
	lg := &i.DeepSeekLegacy
	if strings.TrimSpace(lg.ChatCompletionsEndpoint) == "" {
		lg.ChatCompletionsEndpoint = "https://api.deepseek.com/chat/completions"
	}
	if strings.TrimSpace(lg.Model) == "" {
		lg.Model = "deepseek-v4-pro"
	}
}
