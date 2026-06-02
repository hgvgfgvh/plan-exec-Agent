package officeDomain

import (
	"AgentTest/behavior/skill"
	"AgentTest/config"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type WordGenSkill struct{}

func (s *WordGenSkill) workDir() string {
	return config.Get().ResolvedPaths().Word
}

func (s *WordGenSkill) Name() string {
	return "Generate_Word_Report"
}

func (s *WordGenSkill) Description() string {
	return "生成 Word 文档（已修复中文乱码问题）"
}

func (s *WordGenSkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(args) < 2 {
		return nil, fmt.Errorf("Generate_Word_Report 需要 2 个参数: content, file_name")
	}

	content, ok1 := args[0].(string)
	fileName, ok2 := args[1].(string)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("参数格式错误")
	}

	if _, err := os.Stat(s.workDir()); os.IsNotExist(err) {
		_ = os.MkdirAll(s.workDir(), 0755)
	}

	if !strings.HasSuffix(strings.ToLower(fileName), ".docx") {
		fileName += ".docx"
	}
	savePath := filepath.Join(s.workDir(), fileName)

	// --- 核心修复：带 BOM 的 UTF-8 写入 ---
	tempTxt := filepath.Join(s.workDir(), "temp_content.txt")
	// 添加 UTF-8 BOM 头: 0xEF, 0xBB, 0xBF
	// 这让 Windows 的 Get-Content 能够无误地识别编码
	utf8WithBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	err := os.WriteFile(tempTxt, utf8WithBOM, 0644)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempTxt)

	// --- 核心修复：PowerShell 指定 Encoding ---
	psScript := fmt.Sprintf(`
$word = New-Object -ComObject Word.Application
$word.Visible = $false
$doc = $word.Documents.Add()
$selection = $word.Selection
$content = Get-Content "%s" -Raw -Encoding UTF8
$selection.TypeText($content)
$doc.SaveAs([ref]"%s")
$doc.Close()
$word.Quit()
[System.Runtime.Interopservices.Marshal]::ReleaseComObject($word)
`, tempTxt, savePath)

	log.Printf("[Skill] 正在生成 Word (解决乱码): %s", savePath)

	// 设置 PowerShell 进程的输入编码为 UTF8
	cmd := exec.CommandContext(ctx, "powershell", "-Command", psScript)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return []interface{}{"fail"}, fmt.Errorf("PowerShell 错误: %v, 输出: %s", err, string(output))
	}

	log.Printf("[Skill] Word 文档已生成且编码正确: %s", savePath)
	return []interface{}{savePath}, nil
}

func init() {
	skill.GlobalManager.Regist(&WordGenSkill{})
}
