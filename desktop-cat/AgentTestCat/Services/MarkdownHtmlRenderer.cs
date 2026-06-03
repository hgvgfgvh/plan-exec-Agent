using System.Net;
using Markdig;

namespace AgentTestCat.Services;

/// <summary>将 Markdown 渲染为带 WebUI 风格样式的 HTML 页面（供 WebView2 展示）。</summary>
public static class MarkdownHtmlRenderer
{
    private static readonly MarkdownPipeline Pipeline = new MarkdownPipelineBuilder()
        .UseAdvancedExtensions()
        .Build();

    private const string Css = """
:root {
  --text: #f0eeeb;
  --text-muted: #a8a4a0;
  --text-faint: #807870;
  --border: #4a4640;
  --accent: #c4a882;
  --surface: #302e3a;
  --radius: 8px;
}
* { box-sizing: border-box; }
html, body {
  margin: 0; padding: 0;
  background: #2a2834;
  color: var(--text);
  font-family: "Segoe UI", "Microsoft YaHei UI", sans-serif;
  font-size: 13px;
  line-height: 1.6;
  max-width: 100%;
  overflow-x: hidden;
}
body { padding: 4px 2px 12px 4px; overflow-y: auto; }
.md-body { line-height: 1.6; color: var(--text); overflow-wrap: anywhere; word-break: break-word; max-width: 100%; }
.md-body > :first-child { margin-top: 0; }
.md-body > :last-child { margin-bottom: 0; }
.md-body h1, .md-body h2, .md-body h3, .md-body h4 {
  margin: 0.75em 0 0.4em; font-weight: 600; line-height: 1.3;
}
.md-body h1 { font-size: 1.35rem; }
.md-body h2 { font-size: 1.2rem; }
.md-body h3 { font-size: 1.05rem; color: #d4d4dc; }
.md-body h4 { font-size: 1rem; color: #b8b8c4; }
.md-body p { margin: 0.5em 0; }
.md-body ul, .md-body ol { margin: 0.45em 0; padding-left: 1.4em; }
.md-body li { margin: 0.25em 0; }
.md-body pre {
  margin: 0.6em 0; padding: 0.85rem 1rem; border-radius: var(--radius);
  border: 1px solid var(--border); background: #121216;
  overflow-x: hidden; white-space: pre-wrap; word-break: break-all;
  font-size: 0.86rem; line-height: 1.45;
}
.md-body code {
  font-family: ui-monospace, Consolas, monospace; font-size: 0.9em;
}
.md-body :not(pre) > code {
  padding: 0.15em 0.4em; border-radius: 5px;
  background: var(--surface); color: #f0abfc;
}
.md-body pre code { color: var(--text); background: transparent; padding: 0; }
.md-body blockquote {
  margin: 0.5em 0; padding: 0.3em 0 0.3em 0.85em;
  border-left: 3px solid var(--accent); color: var(--text-muted);
}
.md-body hr { border: none; border-top: 1px solid var(--border); margin: 0.85em 0; }
.md-body a { color: var(--accent); text-decoration: none; }
.md-body a:hover { text-decoration: underline; }
.md-body strong { color: #f4f4f8; font-weight: 600; }
.md-body table {
  width: 100%; border-collapse: collapse; font-size: 0.88rem;
  margin: 0.65em 0; border: 1px solid var(--border); background: #121216;
  table-layout: fixed;
}
.md-body th, .md-body td {
  padding: 0.5rem 0.75rem; text-align: left;
  border-bottom: 1px solid var(--border); vertical-align: top;
  overflow-wrap: anywhere; word-break: break-word;
}
.md-body th { font-weight: 600; color: #f0f0f5; background: rgba(255,255,255,0.04); }
.md-body tr:last-child td { border-bottom: none; }
.md-footer {
  margin-top: 0.65em; padding-top: 0.5em;
  border-top: 1px dashed var(--border);
  font-size: 0.8rem; color: var(--text-muted); white-space: pre-wrap;
}
""";

    public static string ToHtmlPage(string source, string markdown)
    {
        var body = MarkdownHelper.BuildMarkdownForRender(source, markdown ?? "");
        var htmlBody = Markdown.ToHtml(body, Pipeline);
        var footer = "";
        var split = MarkdownHelper.SplitPlanFooter(markdown ?? "");
        if (!string.IsNullOrWhiteSpace(split.Footer) &&
            MarkdownHelper.ShouldUseMarkdown(source, markdown ?? ""))
        {
            footer = "<div class=\"md-footer\">" + WebUtility.HtmlEncode(split.Footer.Trim()) + "</div>";
        }

        return "<!DOCTYPE html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"/>"
               + "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"/>"
               + "<style>" + Css + "</style></head><body><div class=\"md-body\">"
               + htmlBody + "</div>" + footer + "</body></html>";
    }

    public static string ToConversationHtml(IEnumerable<Models.ChatMessage> messages)
    {
        var sb = new System.Text.StringBuilder();
        sb.Append("<!DOCTYPE html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"/>");
        sb.Append("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"/>");
        sb.Append("<style>").Append(Css);
        sb.Append(".msg { margin: 0.65em 0; padding: 0.55em 0.65em; border-radius: 8px; overflow-wrap: anywhere; word-break: break-word; max-width: 100%; }");
        sb.Append(".msg--user { background: rgba(196,168,130,0.15); border-left: 3px solid #c4a882; }");
        sb.Append(".msg--assistant { background: rgba(255,255,255,0.04); border-left: 3px solid #807870; }");
        sb.Append(".msg--system { background: rgba(180,80,80,0.12); font-size: 0.9em; color: #a8a4a0; }");
        sb.Append(".msg-label { font-size: 0.75rem; color: #a8a4a0; margin-bottom: 0.25em; display: block; }");
        sb.Append(".msg.streaming { opacity: 0.85; }");
        sb.Append("</style></head><body><div class=\"md-body\">");

        foreach (var m in messages)
        {
            var cls = m.IsUser ? "msg msg--user" : m.Source == "系统" ? "msg msg--system" : "msg msg--assistant";
            if (m.IsStreaming) cls += " streaming";
            var label = WebUtility.HtmlEncode(m.IsUser ? "你" : m.Source);
            sb.Append("<div class=\"").Append(cls).Append("\">");
            sb.Append("<span class=\"msg-label\">").Append(label);
            if (m.IsStreaming) sb.Append(" · 生成中…");
            sb.Append("</span>");

            try
            {
                if (!m.IsUser && MarkdownHelper.ShouldUseMarkdown(m.Source, m.Text))
                    sb.Append(Markdown.ToHtml(MarkdownHelper.BuildMarkdownForRender(m.Source, m.Text), Pipeline));
                else
                    sb.Append("<p>").Append(WebUtility.HtmlEncode(m.Text).Replace("\n", "<br/>")).Append("</p>");
            }
            catch
            {
                sb.Append("<p>").Append(WebUtility.HtmlEncode(m.Text).Replace("\n", "<br/>")).Append("</p>");
            }

            sb.Append("</div>");
        }

        sb.Append("</div></body></html>");
        return sb.ToString();
    }
}
