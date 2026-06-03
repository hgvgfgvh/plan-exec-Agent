namespace AgentTestCat.Services;

/// <summary>
/// 与 WebUI app.js 对齐：流式合并 message_id、忽略运行视图 JSON、优先展示编排正文。
/// </summary>
public sealed class SseBubbleAggregator
{
    private static readonly HashSet<string> AssistantSources = new(StringComparer.Ordinal)
    {
        "计划编排", "反馈", "行为编排", "Agent"
    };

    private static readonly HashSet<string> StatusSources = new(StringComparer.Ordinal)
    {
        "丘脑", "计划进度"
    };

    private readonly Dictionary<string, StreamState> _streams = new(StringComparer.Ordinal);
    private int _turnBestPriority;

    private sealed class StreamState
    {
        public string Source { get; set; } = "";
        public string Text { get; set; } = "";
    }

    public void BeginTurn() => ResetTurn();

    public void ResetTurn()
    {
        _streams.Clear();
        _turnBestPriority = 0;
    }

    /// <summary>处理 SSE 条目；若应更新气泡则返回 true。</summary>
    public bool TryUpdate(OutputEvent entry, out string source, out string text, out bool isFinal)
    {
        source = "";
        text = "";
        isFinal = false;

        var src = (entry.Source ?? "").Trim();
        if (string.IsNullOrEmpty(src))
            src = "Agent";

        if (src == "运行视图" || LooksLikeRunViewJson(entry.Text))
            return false;

        var eventType = (entry.Event ?? "").Trim();
        var msgId = (entry.MessageId ?? "").Trim();

        if (!string.IsNullOrEmpty(msgId) && eventType is "delta" or "final")
            return HandleStreamEntry(src, msgId, eventType, entry.Text ?? "", out source, out text, out isFinal);

        return HandlePlainEntry(src, entry.Text ?? "", out source, out text, out isFinal);
    }

    private bool HandleStreamEntry(
        string src,
        string msgId,
        string eventType,
        string chunk,
        out string source,
        out string text,
        out bool isFinal)
    {
        source = "";
        text = "";
        isFinal = false;

        if (!_streams.TryGetValue(msgId, out var state))
        {
            state = new StreamState { Source = src };
            _streams[msgId] = state;
        }

        if (eventType == "delta" && chunk.Length > 0)
            state.Text += chunk;
        else if (eventType == "final")
        {
            if (chunk.Length > 0)
                state.Text += chunk;
            _streams.Remove(msgId);
            isFinal = true;
        }

        var priority = SourcePriority(state.Source);
        if (priority <= 0)
            return false;

        if (string.IsNullOrWhiteSpace(state.Text) && !isFinal)
            return false;

        _turnBestPriority = Math.Max(_turnBestPriority, priority);
        source = state.Source;
        text = state.Text.TrimEnd();
        return !string.IsNullOrWhiteSpace(text) || isFinal;
    }

    private bool HandlePlainEntry(
        string src,
        string body,
        out string source,
        out string text,
        out bool isFinal)
    {
        source = "";
        text = "";
        isFinal = true;

        body = body.Trim();
        if (string.IsNullOrEmpty(body))
            return false;

        if (LooksLikeRunViewJson(body))
            return false;

        var priority = SourcePriority(src);
        if (priority <= 0)
            return false;

        if (priority < _turnBestPriority)
            return false;

        _turnBestPriority = Math.Max(_turnBestPriority, priority);
        source = src;
        text = body;
        return true;
    }

    private static int SourcePriority(string source)
    {
        if (source == "运行视图") return 0;
        if (AssistantSources.Contains(source)) return 2;
        if (StatusSources.Contains(source)) return 1;
        if (source is "系统" or "系统异常") return 1;
        return 1;
    }

    private static bool LooksLikeRunViewJson(string? text)
    {
        var t = (text ?? "").Trim();
        if (!t.StartsWith('{') || !t.EndsWith('}')) return false;
        return t.Contains("\"turn_id\"", StringComparison.Ordinal)
               && t.Contains("\"status\"", StringComparison.Ordinal)
               && (t.Contains("\"html_url\"", StringComparison.Ordinal) || t.Contains("html_url", StringComparison.Ordinal));
    }
}
