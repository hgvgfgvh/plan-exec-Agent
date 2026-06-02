package active

import (
	"AgentTest/behavior/skill"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ScanUISkill 对应 YAML 中的 ScanActionableElements 技能
type ScanUISkill struct{}

func (s *ScanUISkill) Name() string { return "ScanActionableElements" }
func (s *ScanUISkill) Description() string {
	return "深度扫描当前焦点窗口及任务栏中的可操作控件，返回中心点坐标"
}

func (s *ScanUISkill) Execute(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// 1. 上下文预检
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 2. PowerShell 脚本定义 (嵌入白名单与中心点计算逻辑)
	psScript := `
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Add-Type -AssemblyName UIAutomationClient
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class Win32 {
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();
}
"@

$actionableTypes = @(
    "ControlType.Button", "ControlType.MenuItem", "ControlType.TabItem", 
    "ControlType.Hyperlink", "ControlType.Slider", "ControlType.CheckBox", 
    "ControlType.RadioButton", "ControlType.Edit", "ControlType.ListItem",
    "ControlType.ComboBox", "ControlType.TreeItem", "ControlType.Thumb"
)

function Get-DeepElements($root, $label, $depth) {
    if ($depth -gt 10) { return } 
    try {
        $children = $root.FindAll([Windows.Automation.TreeScope]::Children, [Windows.Automation.Condition]::TrueCondition)
        foreach($el in $children) {
            $cur = $el.Current
            $type = $cur.ControlType.ProgrammaticName
            if ($cur.IsOffscreen -eq $false) {
                if ($actionableTypes -contains $type) {
                    $rect = $cur.BoundingRectangle
                    if ($rect.Width -gt 0 -and $rect.Height -gt 0 -and (-not [string]::IsNullOrWhiteSpace($cur.Name))) {
                        $cX = [Math]::Round($rect.X + ($rect.Width / 2))
                        $cY = [Math]::Round($rect.Y + ($rect.Height / 2))
                        Write-Host "ITEM>>[$label]$($cur.Name)|$($type)|$($cX),$($cY)"
                    }
                }
                Get-DeepElements -root $el -label $label -depth ($depth + 1)
            }
        }
    } catch {}
}

$fgHwnd = [Win32]::GetForegroundWindow()
if ($fgHwnd -ne [IntPtr]::Zero) {
    $fgElement = [Windows.Automation.AutomationElement]::FromHandle($fgHwnd)
    Get-DeepElements -root $fgElement -label "Focus" -depth 0
}

$taskbar = [Windows.Automation.AutomationElement]::RootElement.FindFirst([Windows.Automation.TreeScope]::Children, (New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::ClassNameProperty, "Shell_TrayWnd")))
if ($taskbar) { Get-DeepElements -root $taskbar -label "Taskbar" -depth 8 }
`

	// 3. 执行物理探测
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	var out bytes.Buffer
	cmd.Stdout = &out

	// 使用带超时的 Context 执行命令
	err := cmd.Run()
	if err != nil {
		return []interface{}{nil, "fail"}, fmt.Errorf("PowerShell 执行失败: %v", err)
	}

	// 4. 解析结果
	var elements []string
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ITEM>>") {
			elements = append(elements, strings.TrimPrefix(line, "ITEM>>"))
		}
	}

	// 5. 返回符合 Schema 的结果
	status := "success"

	return []interface{}{elements, status}, nil
}

func init() {
	skill.GlobalManager.Regist(&ScanUISkill{})
}
