package runview

import (
	"regexp"
	"strings"
)

var (
	reScriptBlock = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	reScriptOpen  = regexp.MustCompile(`(?is)<script\b[^>]*>`)
	reOnAttr      = regexp.MustCompile(`(?is)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reJavascript  = regexp.MustCompile(`(?is)javascript\s*:`)
)

// SanitizeHTML 剥离常见 XSS 向量；运行视图在 iframe sandbox 中展示，仍做基础清洗。
func SanitizeHTML(html string) string {
	s := strings.TrimSpace(html)
	if s == "" {
		return fallbackHTML("（模型未返回内容）")
	}
	s = reScriptBlock.ReplaceAllString(s, "")
	s = reScriptOpen.ReplaceAllString(s, "")
	s = reOnAttr.ReplaceAllString(s, "")
	s = reJavascript.ReplaceAllString(s, "")
	return s
}

func fallbackHTML(reason string) string {
	return `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8"/><title>运行视图</title>
<style>body{font-family:system-ui,sans-serif;background:#18181c;color:#ececf1;padding:1.5rem;line-height:1.6}
h1{font-size:1.1rem}section{margin-top:1rem;padding:1rem;border:1px solid rgba(255,255,255,.1);border-radius:8px}</style></head>
<body><h1>运行视图</h1><section>` + escapeHTML(reason) + `</section></body></html>`
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
