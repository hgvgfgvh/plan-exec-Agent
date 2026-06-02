package runview

import (
	"AgentTest/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HTMLFileForTurn 返回已生成的 HTML 绝对路径；不存在则错误。
func HTMLFileForTurn(turnID string) (string, error) {
	if err := validateTurnID(turnID); err != nil {
		return "", err
	}
	s := loadSettings()
	p := htmlPath(s.OutputDir, turnID)
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}

// ResolveArtifactFile 根据 manifest 解析产物绝对路径（须在 workspace 下）。
func ResolveArtifactFile(turnID, artifactID string) (string, error) {
	if err := validateTurnID(turnID); err != nil {
		return "", err
	}
	artifactID = strings.TrimSpace(artifactID)
	if artifactID == "" {
		return "", fmt.Errorf("empty artifact_id")
	}
	s := loadSettings()
	m, err := readManifest(manifestPath(s.OutputDir, turnID))
	if err != nil {
		return "", err
	}
	var rel string
	for _, a := range m.Artifacts {
		if a.ID == artifactID {
			rel = strings.TrimSpace(a.Path)
			break
		}
	}
	if rel == "" {
		return "", fmt.Errorf("artifact not found: %s", artifactID)
	}
	return resolveUnderWorkspace(rel)
}

func resolveUnderWorkspace(p string) (string, error) {
	cfg := config.TryGet()
	if cfg == nil {
		return "", fmt.Errorf("config unavailable")
	}
	root := cfg.ResolvedPaths().Workspace
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cfg.AbsRoot(), filepath.Clean(p))
	}
	abs = filepath.Clean(abs)
	ws := filepath.Clean(root)
	if !strings.HasPrefix(abs, ws+string(os.PathSeparator)) && abs != ws {
		return "", fmt.Errorf("path outside workspace")
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}
