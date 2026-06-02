package runview

import (
	"AgentTest/config"
	"path/filepath"
	"strings"
)

// Settings 从 config 读取的运行视图配置（旁路模块专用；LLM 与 agents.models 无关）。
type Settings struct {
	Enabled        bool
	LLMAPIBase     string
	LLMAPIKey      string
	LLMModel       string
	LLMTimeoutSec  int
	TurnLogDir     string // 绝对路径
	OutputDir      string // 绝对路径
	MaxBundleRunes int
	DebounceMs     int
}

func loadSettings() Settings {
	s := Settings{
		Enabled:        true,
		MaxBundleRunes: 12000,
		DebounceMs:     600,
		LLMTimeoutSec:  180,
	}
	cfg := config.TryGet()
	if cfg == nil {
		return s
	}
	rv := cfg.RunView
	s.Enabled = rv.Enabled
	s.LLMAPIBase = strings.TrimSpace(rv.LLMAPIBase)
	s.LLMAPIKey = strings.TrimSpace(rv.LLMAPIKey)
	s.LLMModel = strings.TrimSpace(rv.LLMModel)
	root := cfg.AbsRoot()
	out := strings.TrimSpace(rv.OutputDir)
	if out == "" {
		out = "WorkSpace/run_views"
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, filepath.Clean(out))
	}
	s.OutputDir = filepath.Clean(out)

	turnDir := strings.TrimSpace(rv.TurnLogDir)
	if turnDir == "" {
		turnDir = filepath.Join(strings.TrimSpace(cfg.Paths.Workspace), "logs", "turns")
		if !filepath.IsAbs(turnDir) {
			turnDir = filepath.Join(root, filepath.Clean(turnDir))
		}
	} else if !filepath.IsAbs(turnDir) {
		turnDir = filepath.Join(root, filepath.Clean(turnDir))
	}
	s.TurnLogDir = filepath.Clean(turnDir)
	if rv.MaxBundleRunes > 0 {
		s.MaxBundleRunes = rv.MaxBundleRunes
	}
	if rv.DebounceMs > 0 {
		s.DebounceMs = rv.DebounceMs
	}
	if rv.LLMTimeoutSec > 0 {
		s.LLMTimeoutSec = rv.LLMTimeoutSec
	}
	return s
}
