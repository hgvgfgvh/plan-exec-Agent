package see

import (
	"AgentTest/behavior/skill"
	"AgentTest/body/eyes"
	"AgentTest/util/VisualAnalysisTool"
	"context"
	"fmt"
	"os"
)

// SeeAndOCRSkill 对应文字与表格结构化提取技能
type SeeAndOCRSkill struct{}

func (s *SeeAndOCRSkill) Name() string { return "SeeAndOCR" }

func (s *SeeAndOCRSkill) Description() string {
	return "文字提取：识别屏幕上的所有文字、表格和表单，保持原有结构"
}

func (s *SeeAndOCRSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 处理可选参数 A_area (默认值为 "all")
	area := "all"
	if len(args) > 0 && args[0] != nil {
		if a, ok := args[0].(string); ok && a != "" {
			area = a
		}
	}

	fmt.Printf("🔍 正在执行 OCR 文字识别，目标区域: [%s]...\n", area)

	// 2. 获取 PC 视觉通道快照
	pcEye := eyes.GetManager().Viewports["PC"]
	if pcEye == nil {
		return nil, fmt.Errorf("系统视觉通道 [PC] 未初始化")
	}

	// 导出图片 (OCR 识别对清晰度要求极高，通常保持原图不压缩)
	tempPath, err := pcEye.GetProcessedCopy(0, 0, false)
	if err != nil {
		return nil, fmt.Errorf("从内存提取图片用于 OCR 失败: %w", err)
	}

	// 3. 构造 OCR 专用的结构化 Prompt
	// 引导模型使用 Markdown 格式，这样 AI 决策链可以轻松识别表格
	prompt := "请识别这张截图中的所有文字。"
	if area != "all" {
		prompt = fmt.Sprintf("请重点识别图中 [%s] 区域内的内容。", area)
	}
	prompt += " 要求：\n1. 保持原有排版结构。\n2. 发现表格请使用 Markdown Table 格式输出。\n3. 发现表单请以 'Key: Value' 格式列出。\n4. 直接输出识别内容，不要包含任何前导词（如 '好的，我为你识别到...'）。"

	// 4. 调用视觉分析工具 (此时后台模型会切换到 OCR 优化模式或高分辨率模型)
	ocrResult, err := VisualAnalysisTool.AnalyzeImageSimple(tempPath, prompt)
	if err != nil {
		return nil, fmt.Errorf("OCR 识别请求失败: %w", err)
	}

	// 5. 打印识别摘要
	fmt.Printf("📝 OCR 识别完成，提取内容长度: %d\n", len(ocrResult))
	defer os.Remove(tempPath)
	// 返回结构化字符串给 AI 决策链
	return []interface{}{ocrResult}, nil
}

func init() {
	// 注册技能
	skill.GlobalManager.Regist(&SeeAndOCRSkill{})
}
