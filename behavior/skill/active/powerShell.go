package active

import (
	"AgentTest/behavior/skill"
	"AgentTest/config"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Windows 盘符绝对路径（X:\ 或 X:/），用于安全校验；避免把 Python 等语言里的冒号误判为路径。
var winDriveAbsPath = regexp.MustCompile(`[A-Za-z]:[\\/]`)

// windowsAbsPathOutsideWorkspace 若在命令中发现盘符绝对路径，且该路径不是以配置工作区为前缀（含边界），则视为越权。
func windowsAbsPathOutsideWorkspace(cmdUpper, workspaceAbs string) bool {
	ws := strings.ToUpper(filepath.Clean(workspaceAbs))
	for _, loc := range winDriveAbsPath.FindAllStringIndex(cmdUpper, -1) {
		rest := cmdUpper[loc[0]:]
		if !strings.HasPrefix(rest, ws) {
			return true
		}
		if len(rest) == len(ws) {
			continue
		}
		next := rest[len(ws)]
		if next != '\\' && next != '/' {
			// 例如工作区 C:\A 与路径 C:\AB — 前缀命中但下一字符不是路径分隔符
			return true
		}
	}
	return false
}

// PowerShell 对应 YAML 中的 PowerShell 技能
type PowerShell struct{}

func (s *PowerShell) Name() string { return "PowerShell" }

func (s *PowerShell) Description() string {
	return "直接在权限文件夹中直接运行Powershell命令(灵活使用：控制文件，获取应用列表，打开应用 等等)"
}

// DecodeGBK 处理 Windows PowerShell 的中文乱码输出
func (s *PowerShell) DecodeGBK(b []byte) string {
	decoder := simplifiedchinese.GBK.NewDecoder()
	ret, err := decoder.Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(ret)
}

func (s *PowerShell) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. 参数校验 (对应 YAML 中的 cond 参数)
	if len(args) < 1 {
		return nil, fmt.Errorf("PowerShell 技能需要一个命令参数 'cond'")
	}
	command, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("命令参数格式错误，预期为 string")
	}

	// 3. 工作空间（配置文件 paths.workspace）
	absWorkSpace := config.Get().ResolvedPaths().Workspace

	// 确保工作空间存在
	if _, err := os.Stat(absWorkSpace); os.IsNotExist(err) {
		os.MkdirAll(absWorkSpace, 0755)
	}

	fmt.Printf("PowerShell:%s", command)
	// 4. 安全审查（不再用「任意冒号」启发式：Python 的 range(1, 11): 等会误伤）
	upperCmd := strings.ToUpper(command)
	if strings.Contains(upperCmd, "..") ||
		windowsAbsPathOutsideWorkspace(upperCmd, absWorkSpace) ||
		strings.Contains(upperCmd, "$ENV") {
		return nil, fmt.Errorf("安全拦截：检测到非法路径访问或系统环境操作！")
	}

	// 5. 构造并执行命令
	// 使用 -LiteralPath 防止路径特殊字符，Set-Location 强制锁定工作区
	secureCmd := fmt.Sprintf("Set-Location -LiteralPath '%s'; %s", absWorkSpace, command)

	// 使用 context 控制进程生命周期，防止命令长时间卡死
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", secureCmd)
	cmd.Dir = absWorkSpace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 6. 组装返回结果 (对应 YAML 中的 output_schema)
	// 如果有错误输出，优先返回错误信息
	var result string
	if err != nil {
		result = fmt.Sprintf("Error: %s; Stderr: %s", err.Error(), s.DecodeGBK(stderr.Bytes()))
	} else {
		result = s.DecodeGBK(stdout.Bytes())
	}

	// 返回结果切片，第一个元素通常映射为 status/content
	return []interface{}{result}, nil
}

func init() {
	skill.GlobalManager.Regist(&PowerShell{})
}
