package experience

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ExperienceEpisode 存储单元结构
type ExperienceEpisode struct {
	Requirement string `json:"query"`      // 原始需求
	SkillTree   string `json:"skill_tree"` // 对应的 SkillTree 结构
	Timestamp   int64  `json:"t"`          // 时间戳
}

type FileExperienceManager struct {
	filePath string
	mu       sync.RWMutex
}

// NewFileExperienceManager 初始化经验管理器
func NewFileExperienceManager(savePath string) *FileExperienceManager {
	return &FileExperienceManager{
		filePath: savePath,
	}
}

// StoreExperience 将“需求-SkillTree”对以 JSON 行形式追加到文件中
func (m *FileExperienceManager) StoreExperience(ctx context.Context, query, skillTree string) error {
	if query == "" || skillTree == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	episode := ExperienceEpisode{
		Requirement: query,
		SkillTree:   skillTree,
		Timestamp:   time.Now().Unix(),
	}

	// 以追加模式打开文件，不存在则创建
	f, err := os.OpenFile(m.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open experience file: %v", err)
	}
	defer f.Close()

	data, err := json.Marshal(episode)
	if err != nil {
		return fmt.Errorf("failed to marshal experience: %v", err)
	}

	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}

	return nil
}

// RetrieveExperience 从 JSON 文件中逆序检索匹配需求的 SkillTree
func (m *FileExperienceManager) RetrieveExperience(ctx context.Context, query string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, err := os.Open(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	var episodes []ExperienceEpisode
	decoder := json.NewDecoder(file)
	for {
		var ep ExperienceEpisode
		if err := decoder.Decode(&ep); err == io.EOF {
			break
		} else if err != nil {
			continue // 跳过损坏的行
		}
		episodes = append(episodes, ep)
	}

	// 逆序查找：优先返回最近一次成功的经验
	for i := len(episodes) - 1; i >= 0; i-- {
		if strings.Contains(episodes[i].Requirement, query) {
			// 直接返回检索到的 SkillTree
			return episodes[i].SkillTree, nil
		}
	}

	return "", nil
}
