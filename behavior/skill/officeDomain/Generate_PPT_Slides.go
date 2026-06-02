package officeDomain

//
//import (
//	"AgentTest/behavior/skill"
//	"context"
//	"encoding/json"
//	"fmt"
//	"os"
//	"path/filepath"
//	"strings"
//
//	"github.com/unidoc/unioffice/presentation"
//)
//
//type PPTGenSkill struct {
//	WorkDir string
//}
//
//func NewPPTGenSkill() *PPTGenSkill {
//	return &PPTGenSkill{
//		WorkDir: `C:\DATA\GODATA\AgentTest\WorkSpace\ppt`,
//	}
//}
//
//func (s *PPTGenSkill) Name() string { return "Generate_PPT_Slides" }
//
//func (s *PPTGenSkill) Description() string {
//	return "根据大纲逻辑生成多页 PPT 幻灯片，保存在指定工作空间"
//}
//
//type SlideData struct {
//	Title   string   `json:"title"`
//	Content []string `json:"content"`
//}
//
//func (s *PPTGenSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
//	select {
//	case <-ctx.Done():
//		return nil, ctx.Err()
//	default:
//	}
//
//	if len(args) < 3 {
//		return nil, fmt.Errorf("Generate_PPT_Slides 缺少参数: title, slides_json, file_name")
//	}
//
//	pptTitle, _ := args[0].(string)
//	slidesJson, _ := args[1].(string)
//	fileName, _ := args[2].(string)
//
//	if _, err := os.Stat(s.WorkDir); os.IsNotExist(err) {
//		_ = os.MkdirAll(s.WorkDir, 0755)
//	}
//
//	if !strings.HasSuffix(strings.ToLower(fileName), ".pptx") {
//		fileName += ".pptx"
//	}
//	savePath := filepath.Join(s.WorkDir, fileName)
//
//	var slides []SlideData
//	if err := json.Unmarshal([]byte(slidesJson), &slides); err != nil {
//		return []interface{}{"fail", "JSON 解析失败"}, nil
//	}
//
//	// 1. 初始化 PPT
//	ppt := presentation.New()
//
//	// 2. 处理封面
//	if pptTitle != "" {
//		slide := ppt.AddSlide()
//		// 创建文本框来作为标题
//		tb := slide.AddTextBox()
//		tb.SetPosition(1.0, 1.0) // 设定文本框位置
//		tb.SetSize(6.0, 1.0)     // 设定文本框大小
//		p := tb.AddParagraph()
//		p.SetAlignment(presentation.AlignCenter) // 设置段落居中对齐
//		run := p.AddRun()
//		run.AddText(pptTitle)
//		run.SetFontSize(44) // 封面大标题
//	}
//
//	// 3. 循环生成内容页
//	for _, sData := range slides {
//		slide := ppt.AddSlide()
//
//		// 添加页面标题文本框
//		titleTb := slide.AddTextBox()
//		titleTb.SetPosition(1.0, 1.5) // 设置位置
//		titleTb.SetSize(6.0, 1.0)     // 设置大小
//		titleRun := titleTb.AddParagraph().AddRun()
//		titleRun.AddText(sData.Title)
//		titleRun.SetFontSize(32)
//
//		// 添加正文文本框
//		contentTb := slide.AddTextBox()
//		contentTb.SetPosition(1.0, 2.5) // 设置正文框位置，避免遮挡标题
//		contentTb.SetSize(6.0, 4.0)     // 设置大小
//
//		// 为每一行添加内容
//		for _, line := range sData.Content {
//			p := contentTb.AddParagraph()
//			p.SetBullet(true) // 开启列表圆点
//			p.AddRun().AddText(line)
//		}
//	}
//
//	// 4. 保存文件
//	if err := ppt.SaveToFile(savePath); err != nil {
//		return []interface{}{"fail", err.Error()}, nil
//	}
//
//	fmt.Printf("✅ PPT 已生成至: %s\n", savePath)
//	return []interface{}{savePath}, nil
//}
//
//func init() {
//	skill.GlobalManager.Regist(NewPPTGenSkill())
//}
