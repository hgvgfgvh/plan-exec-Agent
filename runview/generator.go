package runview

import (
	"AgentTest/outputbus"
	"AgentTest/turnjournal"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

const systemPrompt = `你是「运行视图」生成器。根据用户本轮诉求与系统记录的输入/输出/步骤/产物，生成一份独立的 HTML 页面（仅 body 内片段，或完整 <!DOCTYPE html> 文档均可）。

要求：
1. 以用户最关心点为主旨组织版面（能力询问→能力表；执行类→步骤时间线；文件类→产物链接）。
2. 只使用提供的 JSON 事实，禁止编造路径、数据、工具结果。
3. 禁止 <script>、on* 事件、javascript: URL、外链 iframe。
4. 样式用内联 <style>，简洁深色友好；可用 <section>、<details>、<table>、<ul>。
5. 产物链接必须使用：<a href="/api/run-view/file?turn_id=TURN_ID&artifact_id=ART_ID">标签</a>，ART_ID 来自 artifacts_index.id；无产物则不写链接。
6. 不要提及 TodoList 文件路径、turnjournal、内部模块名。
7. 只输出 HTML，不要 markdown 围栏，不要解释。`

// Generate 读取回合日志、调用 LLM、落盘 HTML+manifest，并 SSE 通知前端。
func Generate(ctx context.Context, settings Settings, logPath string) {
	bundle, err := loadBundle(logPath)
	if err != nil || bundle == nil {
		log.Printf("[runview] 读取回合日志失败 %s: %v", logPath, err)
		return
	}
	if bundle.TurnID == "" || bundle.EndedAt.IsZero() {
		return
	}
	turnID := bundle.TurnID
	outputbus.PublishRunView(turnID, "pending", "", "")

	if err := ensureOutputDir(settings.OutputDir); err != nil {
		fail(turnID, err)
		return
	}

	bundleJSON, err := bundleJSONForModel(bundle, settings.MaxBundleRunes)
	if err != nil {
		fail(turnID, err)
		return
	}

	html, genErr := generateHTML(ctx, settings, turnID, bundle.UserInput, bundleJSON)
	if genErr != nil {
		log.Printf("[runview] LLM 生成失败 turn=%s: %v", turnID, genErr)
		html = buildTemplateHTML(bundle)
	}

	html = SanitizeHTML(html)
	htmlPath := htmlPath(settings.OutputDir, turnID)
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		fail(turnID, err)
		return
	}

	manifest := &Manifest{TurnID: turnID, Artifacts: bundle.ArtifactsIndex}
	if err := writeManifest(manifestPath(settings.OutputDir, turnID), manifest); err != nil {
		log.Printf("[runview] manifest: %v", err)
	}

	htmlURL := "/api/run-view/html?turn_id=" + turnID
	outputbus.PublishRunView(turnID, "ready", htmlURL, "")
	fmt.Printf("[runview] 已生成运行视图 turn=%s path=%s\n", turnID, htmlPath)
}

func fail(turnID string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	outputbus.PublishRunView(turnID, "failed", "", msg)
	log.Printf("[runview] failed turn=%s: %v", turnID, err)
}

func loadBundle(path string) (*turnjournal.Bundle, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bundle turnjournal.Bundle
	if err := json.Unmarshal(b, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func bundleJSONForModel(b *turnjournal.Bundle, maxRunes int) (string, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	s := string(raw)
	r := []rune(s)
	if len(r) > maxRunes {
		s = string(r[:maxRunes]) + "…(truncated)"
	}
	return s, nil
}

func generateHTML(ctx context.Context, settings Settings, turnID, userQuery, bundleJSON string) (string, error) {
	user := fmt.Sprintf("turn_id=%s\n用户诉求:\n%s\n\n回合记录 JSON:\n%s\n\n请生成 HTML。产物链接 turn_id 使用: %s",
		turnID, userQuery, bundleJSON, turnID)
	out, err := chatCompletion(ctx, settings, systemPrompt, user)
	if err != nil {
		return "", err
	}
	out = strings.TrimPrefix(out, "```html")
	out = strings.TrimPrefix(out, "```HTML")
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimSuffix(out, "```")
	return strings.TrimSpace(out), nil
}

func buildTemplateHTML(b *turnjournal.Bundle) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8"/><title>运行视图</title>
<style>body{font-family:system-ui,sans-serif;background:#18181c;color:#ececf1;padding:1.25rem;line-height:1.55}
h1{font-size:1.15rem;margin:0 0 .5rem}h2{font-size:.95rem;color:#9b9ba7;margin:1.25rem 0 .5rem}
section,article{padding:.75rem 1rem;margin:.5rem 0;border:1px solid rgba(255,255,255,.1);border-radius:8px}
.ok{color:#4ade80}.err{color:#f87171}a{color:#6e8efb}</style></head><body>`)
	sb.WriteString("<h1>运行视图</h1>")
	sb.WriteString("<section><strong>用户</strong><br/>")
	sb.WriteString(escapeHTML(b.UserInput))
	sb.WriteString("</section>")
	if b.ProcessError != "" {
		sb.WriteString(`<section class="err"><strong>错误</strong><br/>`)
		sb.WriteString(escapeHTML(b.ProcessError))
		sb.WriteString("</section>")
	}
	if strings.TrimSpace(b.Portal.FinalReplyExcerpt) != "" {
		sb.WriteString("<h2>门户答复</h2><section>")
		sb.WriteString(escapeHTML(b.Portal.FinalReplyExcerpt))
		sb.WriteString("</section>")
	}
	if b.Plan != nil && len(b.Plan.Steps) > 0 {
		sb.WriteString("<h2>步骤</h2>")
		for i, st := range b.Plan.Steps {
			sb.WriteString("<article><strong>")
			fmt.Fprintf(&sb, "步骤 %d · %s</strong> [%s]<br/>", i+1, escapeHTML(st.Title), escapeHTML(st.Status))
			if ex := st.ResultExcerpt; ex != "" {
				sb.WriteString(escapeHTML(ex))
			}
			for _, art := range st.Artifacts {
				for _, ref := range b.ArtifactsIndex {
					if ref.Path == art && ref.ID != "" {
						fmt.Fprintf(&sb, `<br/><a href="/api/run-view/file?turn_id=%s&artifact_id=%s">%s</a>`,
							b.TurnID, ref.ID, escapeHTML(ref.Label))
						break
					}
				}
			}
			sb.WriteString("</article>")
		}
	}
	sb.WriteString("</body></html>")
	return sb.String()
}
