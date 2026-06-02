package see

import (
	"AgentTest/behavior/skill"
	"AgentTest/body/eyes"
	"AgentTest/util/VisualAnalysisTool"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	//"github.com/go-vgo/robotgo"
)

type SeePCSkill struct{}

func (s *SeePCSkill) Name() string { return "SeePCByEyesAndGetXY" }
func (s *SeePCSkill) Description() string {
	return "从PC视觉通道获取截图并提取精确坐标。已内置DPI缩放补偿与空间校准。（不要轻易使用 耗时且不够准确）"
}

func (s *SeePCSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	if len(args) == 0 || args[0] == nil {
		return nil, fmt.Errorf("missing target description")
	}
	what, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("参数格式错误")
	}

	// 1. 获取系统逻辑分辨率 (例如: 2560x1440)
	// 这是 RobotGo 点击时识别的坐标系，也是我们要强制 AI 认同的坐标系

	pcEye := eyes.GetManager().Viewports["PC"]
	rawImg := pcEye.GetSnapshot()
	if rawImg == nil {
		return nil, fmt.Errorf("无法获取截图数据")
	}
	// 1. 获取图像的边界矩形 (Rectangle)
	bounds := rawImg.Bounds()

	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()
	// 3. 生成临时文件
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("nexus_vision_resync_%d.png", time.Now().UnixNano()))
	if err := imaging.Save(rawImg, tempPath); err != nil {
		return nil, err
	}
	defer os.Remove(tempPath)

	// 4. 高强度约束 Prompt
	prompt := fmt.Sprintf(
		"你是一个精确的坐标定位助手。当前屏幕分辨率【必须】视为 %dx%d。\n"+
			"图片左上角为 [0,0]，右下角为 [%d,%d]。\n"+
			"请在该图片中定位目标：'%s'。\n"+
			"要求：\n"+
			"1. 输出该目标的中心点像素坐标 [x, y]。\n"+
			"2. 随后进行页面内容的基本描述。",
		imgWidth, imgHeight, imgWidth, imgHeight, what,
	)

	fmt.Println(prompt)
	// 5. 视觉模型分析
	result, err := VisualAnalysisTool.AnalyzeImage(tempPath, prompt)
	if err != nil {
		return nil, fmt.Errorf("视觉分析失败: %w", err)
	}

	fmt.Printf("🎯 视觉同步输出: %s\n", result)
	return []interface{}{result}, nil
}

func init() {
	skill.GlobalManager.Regist(&SeePCSkill{})
}
