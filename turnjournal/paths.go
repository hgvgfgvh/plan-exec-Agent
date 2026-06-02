package turnjournal

import (
	"AgentTest/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultDir = "WorkSpace/logs/turns"

// Dir 返回回合日志目录（不存在则创建）。
func Dir() (string, error) {
	cfg := config.TryGet()
	root := "."
	if cfg != nil {
		root = cfg.AbsRoot()
	}
	dir := defaultDir
	if cfg != nil && strings.TrimSpace(cfg.Paths.Workspace) != "" {
		dir = filepath.Join(strings.TrimSpace(cfg.Paths.Workspace), "logs", "turns")
	}
	abs := dir
	if !filepath.IsAbs(dir) {
		abs = filepath.Join(root, filepath.Clean(dir))
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	return abs, nil
}

// FilePath 返回某 turn_id 对应日志文件的绝对路径。
func FilePath(turnID string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	safe := strings.ReplaceAll(strings.TrimSpace(turnID), string(os.PathSeparator), "_")
	if safe == "" {
		return "", fmt.Errorf("turnjournal: empty turn_id")
	}
	return filepath.Join(dir, safe+".json"), nil
}
