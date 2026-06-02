package capabilities

import (
	"encoding/json"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// ToolParametersSchema 为 API function calling 返回 JSON Schema；MCP 工具尽量用 catalog 内 schema。
func ToolParametersSchema(publicName string) map[string]any {
	if schema := mcpToolInputSchema(publicName); schema != nil {
		return schema
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func mcpToolInputSchema(publicName string) map[string]any {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	for _, e := range mcpCatalog {
		if e.PublicName == publicName && e.InputSchema != nil {
			return e.InputSchema
		}
	}
	return nil
}

// DescribeToolForAPI 供 BuildAPITools 使用的简短描述。
func DescribeToolForAPI(t tools.Tool) string {
	return strings.TrimSpace(t.Description())
}

// ParseInputSchemaJSON 将 MCP InputSchema 对象转为 map。
func ParseInputSchemaJSON(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case map[string]any:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			return m
		}
	}
	return nil
}
