package see

import (
	"AgentTest/behavior/skill"
	"AgentTest/userupload"
	"AgentTest/util/VisualAnalysisTool"
	"context"
	"fmt"
	"strings"
)

// AnalyzeWorkspaceImageSkill 分析用户上传到工作区的图片（非截屏/摄像头）。
type AnalyzeWorkspaceImageSkill struct{}

func (s *AnalyzeWorkspaceImageSkill) Name() string { return "AnalyzeWorkspaceImage" }

func (s *AnalyzeWorkspaceImageSkill) Description() string {
	return "分析工作区内用户上传的图片文件（png/jpg 等），返回描述或 OCR 结果"
}

func (s *AnalyzeWorkspaceImageSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	imagePath, focus, err := resolveWorkspaceImageArgs(args...)
	if err != nil {
		return nil, err
	}
	abs, err := userupload.ResolveUnderWorkspace(imagePath)
	if err != nil {
		return nil, err
	}
	if userupload.KindForName(abs) != "image" {
		return nil, fmt.Errorf("AnalyzeWorkspaceImage 仅支持图片文件: %s", imagePath)
	}

	var prompt string
	if focus != "" {
		prompt = fmt.Sprintf("请分析这张图片，重点关注: %s。请描述画面内容；若主要为文字请尽量完整提取。", focus)
	} else {
		prompt = "请详细描述这张图片的内容；若包含大量文字或表格，请尽量结构化提取。"
	}

	fmt.Printf("🖼️  分析工作区图片: %s\n", abs)
	description, err := VisualAnalysisTool.AnalyzeImageSimple(abs, prompt)
	if err != nil {
		return nil, fmt.Errorf("工作区图片分析失败: %w", err)
	}
	return []interface{}{description}, nil
}

func resolveWorkspaceImageArgs(args ...interface{}) (imagePath, focus string, err error) {
	if len(args) < 1 || args[0] == nil {
		return "", "", fmt.Errorf("AnalyzeWorkspaceImage 需要参数 A_image_path")
	}
	switch v := args[0].(type) {
	case string:
		imagePath = strings.TrimSpace(v)
		if len(args) >= 2 {
			if f, ok := args[1].(string); ok {
				focus = strings.TrimSpace(f)
			}
		}
	case map[string]interface{}:
		imagePath = workspaceImageArgString(v, "A_image_path", "image_path", "path")
		focus = workspaceImageArgString(v, "A_focus", "focus")
	default:
		return "", "", fmt.Errorf("参数格式错误：args[0] 须为路径 string 或含 A_image_path 的对象")
	}
	if imagePath == "" {
		return "", "", fmt.Errorf("参数 A_image_path 不能为空")
	}
	return imagePath, focus, nil
}

func workspaceImageArgString(m map[string]interface{}, keys ...string) string {
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
	skill.GlobalManager.Regist(&AnalyzeWorkspaceImageSkill{})
}
