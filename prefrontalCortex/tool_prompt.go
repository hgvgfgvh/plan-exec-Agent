package prefrontalCortex

import (
	"AgentTest/capabilities"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tmc/langchaingo/tools"
)

const toolBriefMaxRunes = 280

// buildExecutorToolPrompt API 主路径下缩短 ReAct 说明；关闭 API 时退回完整文本工具表。
func buildExecutorToolPrompt(toolMap map[string]tools.Tool) string {
	if UseAPIToolCalls() {
		return buildAPIModeToolPrompt(toolMap)
	}
	return buildToolDocsPrompt(toolMap)
}

func buildAPIModeToolPrompt(toolMap map[string]tools.Tool) string {
	var sb strings.Builder
	sb.WriteString("## 工具调用（API 主路径）\n\n")
	sb.WriteString("模型通过 **tools / tool_calls** 调用下列工具；参数须符合各工具 JSON Schema。\n")
	sb.WriteString("仅在接口不支持 function calling 时，才在正文使用 `Action:` / `Action Input:` 兜底。\n\n")
	sb.WriteString("- **MCP**：公开名见 AGENTS.md；须先 `get_capability_details` 解锁后，对应 MCP 才会出现在 tool_calls 列表中。\n")
	sb.WriteString("- **内置技能**：`SetExecutorStep`，参数 `{\"skill\":\"技能名\",\"args\":[]}`。\n")
	sb.WriteString("- **能力目录**：需要准确列举能力/MCP 时，由你自行 `tool_calls` 调用 `list_agent_capabilities`（无参 `{}`），禁止编造未列出项。\n")
	sb.WriteString("- **Schema 详情**：`get_capability_details` 返回说明并解锁 MCP；解锁前勿直接调用未出现的 MCP 名。\n\n")
	names := sortedToolNamesForPrompt(toolMap)
	for _, name := range names {
		t := toolMap[name]
		sb.WriteString("- `")
		sb.WriteString(name)
		sb.WriteString("`: ")
		sb.WriteString(briefToolDescription(t.Description()))
		sb.WriteByte('\n')
	}
	if hasHiddenMCP(toolMap) {
		sb.WriteString("\nMCP 公开名见 system 文末 **AGENTS.md**；执行前用 get_capability_details 按需解锁。\n")
	}
	return sb.String()
}

func sortedToolNamesForPrompt(toolMap map[string]tools.Tool) []string {
	names := make([]string, 0, len(toolMap))
	for name, t := range toolMap {
		if sup, ok := t.(capabilities.ExecutorToolPromptSuppressor); ok && sup.SuppressExecutorToolPrompt() {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func hasHiddenMCP(toolMap map[string]tools.Tool) bool {
	for _, t := range toolMap {
		if sup, ok := t.(capabilities.ExecutorToolPromptSuppressor); ok && sup.SuppressExecutorToolPrompt() {
			return true
		}
	}
	return false
}

// buildToolDocsPrompt 以统一的 Function Calling 格式列出可调工具（不含 MCP 逐项说明）。
func buildToolDocsPrompt(toolMap map[string]tools.Tool) string {
	names := sortedToolNamesForPrompt(toolMap)
	hiddenMCP := hasHiddenMCP(toolMap)

	var sb strings.Builder
	sb.WriteString("## 工具调用（ReAct + 统一 JSON）\n\n")
	sb.WriteString("```\nAction: <工具名>\nAction Input: <JSON 对象>\n```\n")
	sb.WriteString("禁止 <Action> XML、禁止 DSML tool_calls。无参写 Action Input: {}\n\n")
	sb.WriteString("**MCP**：Action 用 AGENTS.md 公开名；Action Input 为该工具 schema 的**扁平对象**（如 {\"query\":\"SELECT ...\"}）。\n")
	sb.WriteString("**内置技能**：Action 固定为 SetExecutorStep；Action Input 为 {\"skill\":\"技能名\",\"args\":[...]}（args 为数组）。\n")
	sb.WriteString("**统一信封（推荐，减少拼错）**：\n")
	sb.WriteString("- MCP: Action Input: {\"_call\":\"mcp\",\"name\":\"sqlite__list_tables\",\"params\":{}}\n")
	sb.WriteString("- 技能: Action Input: {\"_call\":\"skill\",\"name\":\"SeeAndOCR\",\"args\":[]}\n")
	sb.WriteString("误写 Action: 技能名 时执行器会自动改为 SetExecutorStep。\n\n")
	for _, name := range names {
		t := toolMap[name]
		sb.WriteString("### ")
		sb.WriteString(name)
		sb.WriteByte('\n')
		sb.WriteString(briefToolDescription(t.Description()))
		sb.WriteString("\n\n")
	}
	if hiddenMCP {
		sb.WriteString("### MCP\n")
		sb.WriteString("MCP 公开名见文末 **AGENTS.md**（已列全名）。执行用 `Action: sqlite__list_tables` 等；`get_capability_details` 仅查 Schema，不能代替执行。\n")
		sb.WriteString("**Resend 发信**：`resend__send_email` 的 `to`/`cc`/`bcc` 必须是 **字符串数组**，如 `\"to\":[\"a@b.com\"]`；工具失败或修正参数后须再次输出 `Action: resend__send_email`，勿只回复 JSON 代码块。\n\n")
	}
	return sb.String()
}

func briefToolDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "（无说明）"
	}
	lines := strings.Split(desc, "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(kept) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "【兼容") || strings.HasPrefix(line, "【推荐：结构化 JSON 树】") ||
			strings.Contains(line, "旧字符串 DSL") || strings.HasPrefix(line, "{\"steps\"") {
			break
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, " ")
	if out == "" {
		out = strings.TrimSpace(lines[0])
	}
	if utf8.RuneCountInString(out) > toolBriefMaxRunes {
		r := []rune(out)
		return string(r[:toolBriefMaxRunes]) + "…"
	}
	return out
}
