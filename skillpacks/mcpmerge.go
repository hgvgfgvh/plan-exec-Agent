package skillpacks

import (
	"AgentTest/config"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type mcpYAMLRoot struct {
	Servers []config.MCPServerDef `yaml:"servers"`
}

// ParseMCPServersFromYAML 支持两种写法：1) 根上 `servers:` 列表；2) 单条 server 与 MCPServerDef 同形（无 servers 键）。
func ParseMCPServersFromYAML(data []byte) ([]config.MCPServerDef, error) {
	var root mcpYAMLRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Servers) > 0 {
		return root.Servers, nil
	}
	var single config.MCPServerDef
	if err := yaml.Unmarshal(data, &single); err != nil {
		return nil, err
	}
	if strings.TrimSpace(single.Command) != "" || strings.TrimSpace(single.Endpoint) != "" {
		return []config.MCPServerDef{single}, nil
	}
	return nil, nil
}

// cursorMCPJSON 与 Cursor ~/.cursor/mcp.json 顶层结构兼容。
type cursorMCPJSON struct {
	MCPServers map[string]cursorServerJSON `json:"mcpServers"`
}

type cursorServerJSON struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func ParseMCPServersFromCursorJSON(data []byte) ([]config.MCPServerDef, error) {
	var top cursorMCPJSON
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&top); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	if len(top.MCPServers) == 0 {
		return nil, nil
	}
	out := make([]config.MCPServerDef, 0, len(top.MCPServers))
	for name, s := range top.MCPServers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		def := config.MCPServerDef{Name: name, Enabled: true}
		if strings.TrimSpace(s.URL) != "" {
			def.Transport = "http"
			def.Endpoint = strings.TrimSpace(s.URL)
			def.Headers = s.Headers
		} else {
			def.Command = strings.TrimSpace(s.Command)
			def.Args = append([]string(nil), s.Args...)
			def.Env = s.Env
		}
		if def.Command == "" && def.Endpoint == "" {
			continue
		}
		out = append(out, def)
	}
	return out, nil
}
