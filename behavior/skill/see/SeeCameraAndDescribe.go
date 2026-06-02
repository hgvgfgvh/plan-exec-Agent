package see

import (
	"AgentTest/behavior/skill"
	"AgentTest/body/eyes"
	"AgentTest/util/VisualAnalysisTool"
	"context"
	"fmt"
	"os"
)

// SeeAndDenseCaptionSkill 对应深度视觉分析技能
type SeeCameraAndDescribe struct{}

func (s *SeeCameraAndDescribe) Name() string { return "SeeCameraAndDescribe" }

func (s *SeeCameraAndDescribe) Description() string {
	return "实时观察摄像头画面，描述现实环境中的人物、物体或事件"
}

func (s *SeeCameraAndDescribe) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 处理可选参数 A_focus
	var focus string
	if len(args) > 0 && args[0] != nil {
		if f, ok := args[0].(string); ok && f != "" {
			focus = f
		}
	}

	if focus != "" {
		fmt.Printf("👁️  正在执行摄像头深度视觉分析，重点关注: [%s]...\n", focus)
	} else {
		fmt.Printf("👁️  正在执行摄像头全局视觉深度分析...\n")
	}

	// 2. 获取 PC 视觉通道快照
	pcEye := eyes.GetManager().Viewports["Camera"]
	if pcEye == nil {
		return nil, fmt.Errorf("系统视觉通道 [Camera] 未初始化")
	}

	// 导出图片 (此处保持原图质量，以便模型看清细节)
	tempPath, err := pcEye.GetProcessedCopy(0, 0, false)
	if err != nil {
		return nil, fmt.Errorf("从内存提取图片失败: %w", err)
	}
	// 确保无论发生什么错误，文件都能被清理
	defer func() {
		if _, err := os.Stat(tempPath); err == nil {
			os.Remove(tempPath)
		}
	}()
	// 3. 构造深度分析指令 (Prompt Engineering)
	var prompt string
	if focus != "" {
		prompt = fmt.Sprintf("请详细描述这张截图。请特别关注并深入分析以下内容: '%s'。请描述其位置、状态、相关文字以及周围的上下文环境。", focus)
	} else {
		prompt = "请对这张屏幕截图进行全方位的密集描述（Dense Caption）。包含打开的窗口、任务栏状态、图标分布、活动中的程序以及任何明显的视觉异常。"
	}

	// 4. 调用视觉分析工具
	// 这里的 VisualAnalysisTool 内部通常封装了对 GPT-4o 或 Qwen-VL 等多模态模型的 API 调用
	description, err := VisualAnalysisTool.AnalyzeImageSimple(tempPath, prompt)
	if err != nil {
		return nil, fmt.Errorf("SeeCameraAndDescribe视觉深度分析失败: %w", err)
	}

	// 5. 打印简短摘要（可选）
	fmt.Printf("✅ 分析完成，描述长度: %d 字符\n", len(description))
	defer os.Remove(tempPath)
	// 返回描述内容给 AI 决策链
	return []interface{}{description}, nil
}

func init() {
	// 将技能注册到全局管理器
	skill.GlobalManager.Regist(&SeeCameraAndDescribe{})
}
