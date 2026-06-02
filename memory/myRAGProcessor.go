package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// MemoryEpisode 代表一次完整的交互经验片段
// 增加 Tag 和 Extra 字段，为未来的多模态（视觉/位置）预留空间
type MemoryEpisode struct {
	HumanContent string            `json:"h"`
	AIContent    string            `json:"a"`
	Timestamp    int64             `json:"t,omitempty"`
	Extra        map[string]string `json:"e,omitempty"` // 扩展字段
}

type MyRAGProcessor struct {
	filePath      string
	episodes      []MemoryEpisode
	hotCache      map[string]int
	strongNeurons map[string]string
	threshold     int
	mu            sync.RWMutex // 增加读写锁，保障持久化安全
}

// NewMyRAGProcessor 初始化处理器并从本地加载历史记录
func NewMyRAGProcessor(threshold int, savePath string) *MyRAGProcessor {
	m := &MyRAGProcessor{
		filePath:      savePath,
		episodes:      make([]MemoryEpisode, 0),
		hotCache:      make(map[string]int),
		strongNeurons: make(map[string]string),
		threshold:     threshold,
	}

	// 初始化直觉
	m.strongNeurons["你是谁"] = "我是由 Go 驱动的 AI 助手，拥有基于神经强化的长效记忆系统。"
	m.strongNeurons["开发者"] = "我的核心架构由一位追求极致的开发者构建，使用 Golang 技术。"

	// 加载历史记忆
	m.loadFromFile()
	return m
}

// loadFromFile 从本地磁盘恢复记忆
func (m *MyRAGProcessor) loadFromFile() {
	file, err := os.Open(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return // 文件不存在说明是第一次运行
		}
		fmt.Printf("加载记忆文件失败: %v\n", err)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for {
		var ep MemoryEpisode
		if err := decoder.Decode(&ep); err == io.EOF {
			break
		} else if err != nil {
			fmt.Printf("解析记忆行失败: %v\n", err)
			continue
		}
		m.episodes = append(m.episodes, ep)
	}
	fmt.Printf(">>> [系统恢复] 已从本地文件加载 %d 组经验记忆\n", len(m.episodes))
}

func (m *MyRAGProcessor) StoreMessages(ctx context.Context, messages []llms.ChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	// 1. 预处理数据
	var newEpisodes []MemoryEpisode
	// 生成易于阅读的年月日时间字符串
	currentTimeStr := time.Now().Format("2006-01-02 15:04:05")
	// 同时保留 Unix 时间戳用于程序逻辑（可选）
	timestamp := time.Now().Unix()

	for i := 0; i < len(messages)-1; i += 2 {
		h, a := messages[i], messages[i+1]
		if h.GetType() == llms.ChatMessageTypeHuman && a.GetType() == llms.ChatMessageTypeAI {
			ep := MemoryEpisode{
				HumanContent: h.GetContent(),
				AIContent:    a.GetContent(),
				Timestamp:    timestamp,
				Extra:        make(map[string]string),
			}
			// 将格式化后的时间存入 Extra 字段，AI 在检索到该片段时能直接读取日期
			ep.Extra["date"] = currentTimeStr

			newEpisodes = append(newEpisodes, ep)
		}
	}

	if len(newEpisodes) == 0 {
		return nil
	}

	// 2. 加锁持久化
	m.mu.Lock()
	defer m.mu.Unlock()

	f, err := os.OpenFile(m.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开记忆文件失败: %v", err)
	}
	defer f.Close()

	fmt.Printf("\n======= RAG 经验固化（时间节点：%s） =======\n", currentTimeStr)

	for _, ep := range newEpisodes {
		data, err := json.Marshal(ep)
		if err != nil {
			continue
		}

		if _, err := f.WriteString(string(data) + "\n"); err != nil {
			fmt.Printf("写入磁盘失败: %v\n", err)
			continue
		}

		m.episodes = append(m.episodes, ep)
		fmt.Printf("[固化并存档] >> 问: %s... (时间已记录)\n", truncateString(ep.HumanContent, 20))
	}

	return nil
}

// Retrieve 深度检索
func (m *MyRAGProcessor) Retrieve(ctx context.Context, query string) ([]llms.ChatMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fmt.Printf(">>> [深度检索] 逆序扫描本地数据库查找与 '%s' 相关的上下文...\n", query)

	var results []llms.ChatMessage
	found := false
	const maxContextPairs = 5 // 限制只返回最近的 5 组相关记忆，防止上下文溢出

	// 1. 倒序遍历：从最新的 episodes[len-1] 开始向旧的 episodes[0] 查找
	for i := len(m.episodes) - 1; i >= 0; i-- {
		ep := m.episodes[i]

		// 关键词匹配逻辑
		if strings.Contains(ep.HumanContent, query) || strings.Contains(ep.AIContent, query) {
			// 获取时间字符串
			datePrefix := ""
			if d, ok := ep.Extra["date"]; ok {
				datePrefix = fmt.Sprintf("[记录时间: %s] ", d)
			}

			// 将匹配到的记忆加入结果（注意：因为是倒序找，新记忆会在数组前面）
			results = append(results,
				llms.GenericChatMessage{Role: "human", Content: datePrefix + ep.HumanContent},
				llms.GenericChatMessage{Role: "ai", Content: ep.AIContent},
			)
			found = true
		}

		// 如果已经找到了足够多的相关记忆，提前结束扫描（性能优化）
		if len(results) >= maxContextPairs*2 {
			break
		}
	}

	// 2. 神经强化逻辑
	m.hotCache[query]++
	if m.hotCache[query] >= m.threshold && found {
		// 这里的 results[0] 和 results[1] 就是最近一次匹配到的“问题-回答”对
		m.strongNeurons[query] = fmt.Sprintf("固化直觉: %s -> %s",
			truncateString(results[0].GetContent(), 20),
			truncateString(results[1].GetContent(), 20))
		fmt.Printf(">>> [神经强化] 关键词 '%s' 热度达标，已捕捉最新关联。\n", query)
	}

	return results, nil
}

// GetInstinct 保持不变
func (m *MyRAGProcessor) GetInstinct(query string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.strongNeurons {
		if strings.Contains(query, k) {
			return v, true
		}
	}
	return "", false
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
