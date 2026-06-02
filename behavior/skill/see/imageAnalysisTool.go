package see

import (
	"AgentTest/behavior/skill"
	"AgentTest/prefrontalCortex" // 确保路径正确
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// QwenSearchSkill 对应 YAML 中的 AI_Search_Skill_By_Qwen3-Max
type QwenSearchSkill struct{}

func NewQwenSearchSkill() *QwenSearchSkill {
	return &QwenSearchSkill{}
}

func (s *QwenSearchSkill) Name() string { return "AI_Search_Skill_By_Qwen3-Max" }

func (s *QwenSearchSkill) Description() string {
	return "通过连接高级 AI(Qwen3-Max) 搜索引擎获取经过提炼的实时信息"
}

func (s *QwenSearchSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 检查上下文
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	query, detailLevel, err := resolveSearchArgs(args...)
	if err != nil {
		return nil, err
	}

	// 3. 构造提示词
	systemPrompt := "你是一个具备实时联网搜索能力的助手。请针对用户的提问进行搜索并汇总最新的、准确的信息。"
	if detailLevel == "brief" {
		systemPrompt += "请保持回答简明扼要，直接给出核心结论。"
	} else {
		systemPrompt += "请提供详尽的分析，并在可能的情况下列出数据来源或关键点。"
	}

	log.Printf("🌐 AI 联网搜索开启: [%s] (Level: %s)", query, detailLevel)

	// 4. 执行带有 enable_search 的请求
	answer, err := s.callWithSearch(ctx, query, systemPrompt)
	if err != nil {
		log.Printf("❌ AI 搜索执行失败: %v", err)
		return []interface{}{err.Error()}, nil
	}

	// 5. 组装返回结果
	log.Printf("✅ AI 搜索成功完成")
	return []interface{}{answer}, nil
}

// callWithSearch 核心实现：注入 enable_search 参数
func (s *QwenSearchSkill) callWithSearch(ctx context.Context, input string, systemPrompt string) (string, error) {
	// 执行时再读 config，避免 skill init 早于 config.Load 导致 api_key 为空。
	m := prefrontalCortex.NewQwenModel()

	// 构造 OpenAI 兼容格式但包含阿里云特有参数的请求体
	requestData := map[string]interface{}{
		"model": m.ModelID,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": input},
		},
		"enable_search": true, // 核心：强制开启通义千问联网插件
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", m.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.APIKey)

	// 执行请求 (复用 60s 超时)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal failed: %v", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no content returned from Qwen3-Max")
}

// resolveSearchArgs 兼容 SetExecutorStep 两种入参：args[0]=string 或 args[0]={A_query,B_detail_level}（与 abilities.yml 一致）。
func resolveSearchArgs(args ...interface{}) (query, detailLevel string, err error) {
	detailLevel = "detailed"
	if len(args) < 1 || args[0] == nil {
		return "", "", fmt.Errorf("AI_Search_Skill 缺少必要参数: A_query")
	}
	switch v := args[0].(type) {
	case string:
		query = strings.TrimSpace(v)
		if len(args) >= 2 {
			if dl, ok := args[1].(string); ok && strings.TrimSpace(dl) != "" {
				detailLevel = strings.TrimSpace(dl)
			}
		}
	case map[string]interface{}:
		query = searchArgString(v, "A_query", "query")
		if dl := searchArgString(v, "B_detail_level", "detail_level"); dl != "" {
			detailLevel = dl
		}
	default:
		return "", "", fmt.Errorf("参数格式错误：args[0] 须为非空 string 或含 A_query 的对象")
	}
	if query == "" {
		return "", "", fmt.Errorf("参数 A_query 不能为空")
	}
	return query, detailLevel, nil
}

func searchArgString(m map[string]interface{}, keys ...string) string {
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
	// 注册到全局管理器
	skill.GlobalManager.Regist(NewQwenSearchSkill())
}
