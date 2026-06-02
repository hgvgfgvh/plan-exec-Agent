using System.Text;
using System.Text.RegularExpressions;

namespace AgentTestCat.Services;

/// <summary>与 WebUI md-render.js 对齐的 Markdown 检测与预览。</summary>
public static class MarkdownHelper
{
    private static readonly HashSet<string> MdSources = new(StringComparer.Ordinal)
    {
        "计划编排", "反馈", "行为编排", "计划进度"
    };

    public static bool ShouldUseMarkdown(string source, string text)
    {
        var src = (source ?? "").Trim();
        if (MdSources.Contains(src)) return true;
        if (src is "系统" or "系统异常" or "丘脑" or "你" or "user") return false;
        if (src is "Agent" or "答复" or "编排" or "执行") return true;
        return LooksLikeMarkdown(text);
    }

    public static bool LooksLikeMarkdown(string text)
    {
        var t = (text ?? "").Trim();
        if (t.Length < 12) return false;
        var score = 0;
        if (Regex.IsMatch(t, @"^#{1,6}\s", RegexOptions.Multiline)) score += 2;
        if (Regex.IsMatch(t, @"```[\s\S]*?```")) score += 3;
        if (Regex.IsMatch(t, @"^[-*+]\s+", RegexOptions.Multiline)) score += 1;
        if (Regex.IsMatch(t, @"^\d+\.\s+", RegexOptions.Multiline)) score += 1;
        if (Regex.IsMatch(t, @"\*\*[^*\n]+\*\*")) score += 1;
        if (Regex.IsMatch(t, @"^>\s", RegexOptions.Multiline)) score += 1;
        if (Regex.IsMatch(t, @"^\|.+\|$", RegexOptions.Multiline) && Regex.IsMatch(t, @"\|[\s\-:|]+\|")) score += 2;
        if (Regex.IsMatch(t, @"\|.+\|") && Regex.IsMatch(t, @"\|[-:\s|]{3,}\|")) score += 2;
        if (Regex.IsMatch(t, @"\n#{1,6}\s")) score += 1;
        if (Regex.IsMatch(t, @"`[^`\n]+`") && (score > 0 || Regex.IsMatch(t, @"^#{1,6}\s", RegexOptions.Multiline))) score += 1;
        return score >= 2;
    }

    public static (string Body, string Footer) SplitPlanFooter(string text)
    {
        var re = new Regex(@"\n\n---\n(?:\s*\n)?（编排 ");
        var m = re.Match(text ?? "");
        if (m.Success && m.Index >= 0)
            return (text![..m.Index], text[m.Index..]);
        return (text ?? "", "");
    }

    public static string BuildMarkdownForRender(string source, string text)
    {
        var (body, footer) = SplitPlanFooter(text ?? "");
        if (MdSources.Contains((source ?? "").Trim()) && !string.IsNullOrWhiteSpace(footer))
            return body.TrimEnd() + "\n\n---\n" + footer.TrimStart('\n');
        return text ?? "";
    }

    public static string PreviewPlain(string source, string text, int maxLen = 96)
    {
        var plain = StripSimpleMarkdown(text ?? "");
        plain = plain.Replace('\r', ' ').Replace('\n', ' ').Trim();
        while (plain.Contains("  ", StringComparison.Ordinal))
            plain = plain.Replace("  ", " ", StringComparison.Ordinal);
        if (plain.Length > maxLen)
            plain = plain[..maxLen] + "…";
        var label = string.IsNullOrWhiteSpace(source) ? "" : source.Trim() + "：";
        return label + plain;
    }

    public static bool NeedsExpand(string text) => (text ?? "").Trim().Length > 72;

    private static string StripSimpleMarkdown(string text)
    {
        if (string.IsNullOrEmpty(text)) return "";
        var s = text;
        s = Regex.Replace(s, @"```[\s\S]*?```", " ");
        s = Regex.Replace(s, @"`([^`\n]+)`", "$1");
        s = Regex.Replace(s, @"^#{1,6}\s+", "", RegexOptions.Multiline);
        s = Regex.Replace(s, @"\*\*([^*]+)\*\*", "$1");
        s = Regex.Replace(s, @"\*([^*]+)\*", "$1");
        s = Regex.Replace(s, @"^[-*+]\s+", "", RegexOptions.Multiline);
        s = Regex.Replace(s, @"^\d+\.\s+", "", RegexOptions.Multiline);
        s = Regex.Replace(s, @"^>\s?", "", RegexOptions.Multiline);
        return s;
    }

    public static string EnforceMaxCharsPerLine(string text, int maxPerLine)
    {
        if (string.IsNullOrEmpty(text)) return "";
        var result = new StringBuilder();
        foreach (var rawLine in text.Replace("\r\n", "\n").Split('\n'))
        {
            if (result.Length > 0) result.Append('\n');
            var line = rawLine;
            while (line.Length > maxPerLine)
            {
                result.Append(line.AsSpan(0, maxPerLine));
                result.Append('\n');
                line = line[maxPerLine..];
            }
            result.Append(line);
        }
        return result.ToString();
    }
}
