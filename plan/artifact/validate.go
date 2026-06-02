package artifact

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/config"
	"AgentTest/plan/skillwait"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const minArtifactRunes = 1

// ValidateReportArtifacts 校验 report_step_result 声明的 artifact 路径。
// 空列表跳过；目录仅验存在；普通文件须可读且非占位/非空白。
func ValidateReportArtifacts(rep runcontrol.StepReport) error {
	if !strings.EqualFold(strings.TrimSpace(rep.Status), "ok") {
		return nil
	}
	for _, p := range rep.Artifacts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := resolveArtifactPath(p)
		if err != nil {
			return err
		}
		if err := validateArtifactPath(p, abs); err != nil {
			return err
		}
	}
	return nil
}

// validateArtifactPath 按路径类型验收：目录仅验存在；普通文件验可读且非占位/非空。
func validateArtifactPath(display, abs string) error {
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("artifact 路径不存在: %s", display)
		}
		return fmt.Errorf("artifact 不可访问: %s (%v)", display, err)
	}
	if info.IsDir() {
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("artifact 须为普通文件或目录: %s", display)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("artifact 不可读: %s (%v)", display, err)
	}
	content := strings.TrimSpace(string(data))
	if skillwait.IsPlaceholderSkillSummary(content) {
		return fmt.Errorf("artifact 仍为占位内容，非真实技能/MCP 输出: %s", display)
	}
	if utf8.RuneCountInString(content) < minArtifactRunes {
		return fmt.Errorf("artifact 为空或仅空白，未写入有效内容: %s", display)
	}
	return nil
}

func resolveArtifactPath(p string) (string, error) {
	p = filepath.Clean(p)
	if filepath.IsAbs(p) {
		return p, nil
	}
	cfg := config.TryGet()
	if cfg == nil {
		return p, nil
	}
	root := cfg.AbsRoot()
	if strings.HasPrefix(strings.ToLower(p), strings.ToLower("WorkSpace")) {
		return filepath.Clean(filepath.Join(root, p)), nil
	}
	ws := cfg.ResolvedPaths().Workspace
	return filepath.Clean(filepath.Join(ws, p)), nil
}
