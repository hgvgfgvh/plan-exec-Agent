using System.Text;
using System.Text.Json;
using AgentTestCat.Models;

namespace AgentTestCat.Services;

public sealed class ChatHistory
{
    private readonly List<ChatMessage> _messages = new();
    private readonly Dictionary<string, string> _streamKeys = new(StringComparer.Ordinal);
    private readonly string _savePath;
    private const int MaxMessages = 200;

    public IReadOnlyList<ChatMessage> Messages => _messages;

    public ChatHistory()
    {
        var dir = System.IO.Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
            "AgentTestPCAPPCat");
        System.IO.Directory.CreateDirectory(dir);
        _savePath = System.IO.Path.Combine(dir, "history.json");
        Load();
    }

    public void AddUser(string text)
    {
        text = text.Trim();
        if (string.IsNullOrEmpty(text)) return;
        _messages.Add(new ChatMessage { Source = "你", Text = text, IsUser = true });
        TrimAndSave();
    }

    public void UpsertAssistant(string source, string text, bool isFinal, string? streamKey = null)
    {
        source = (source ?? "").Trim();
        text = text ?? "";

        ChatMessage? msg = null;
        if (!string.IsNullOrWhiteSpace(streamKey) && _streamKeys.TryGetValue(streamKey, out var id))
            msg = _messages.FirstOrDefault(m => m.Id == id);

        if (msg == null)
        {
            if (string.IsNullOrWhiteSpace(text) && isFinal)
                return;
            msg = new ChatMessage { Source = source, Text = text, IsUser = false, IsStreaming = !isFinal };
            _messages.Add(msg);
            if (!string.IsNullOrWhiteSpace(streamKey))
                _streamKeys[streamKey] = msg.Id;
        }
        else
        {
            msg.Source = source;
            if (!string.IsNullOrWhiteSpace(text))
                msg.Text = text;
            msg.IsStreaming = !isFinal;
        }

        if (isFinal && !string.IsNullOrWhiteSpace(streamKey))
            _streamKeys.Remove(streamKey);

        TrimAndSave();
    }

    public void AddSystem(string text)
    {
        if (string.IsNullOrWhiteSpace(text)) return;
        _messages.Add(new ChatMessage { Source = "系统", Text = text.Trim(), IsUser = false });
        TrimAndSave();
    }

    public string CollapsedPreview(int maxLen = 120)
    {
        var last = _messages.LastOrDefault(m => !m.IsUser && m.Source != "系统")
                   ?? _messages.LastOrDefault();
        if (last == null) return "喵～";
        var label = last.IsUser ? "你" : last.Source;
        var plain = MarkdownHelper.PreviewPlain(label, last.Text, maxLen);
        return plain;
    }

    public bool NeedsExpand() =>
        _messages.Count > 1 || _messages.Any(m => m.Text.Length > 72);

    public void Clear()
    {
        _messages.Clear();
        _streamKeys.Clear();
        Save();
    }

    private void TrimAndSave()
    {
        while (_messages.Count > MaxMessages)
            _messages.RemoveAt(0);
        Save();
    }

    private void Load()
    {
        try
        {
            if (!System.IO.File.Exists(_savePath)) return;
            var json = System.IO.File.ReadAllText(_savePath);
            var list = JsonSerializer.Deserialize<List<ChatMessage>>(json);
            if (list == null) return;
            _messages.Clear();
            _messages.AddRange(list);
        }
        catch
        {
            /* ignore corrupt history */
        }
    }

    private void Save()
    {
        try
        {
            var json = JsonSerializer.Serialize(_messages, new JsonSerializerOptions { WriteIndented = false });
            System.IO.File.WriteAllText(_savePath, json, Encoding.UTF8);
        }
        catch
        {
            /* ignore */
        }
    }
}
