namespace AgentTestCat.Services;

/// <summary>连接 AgentTest、SSE、历史与附件。</summary>
public sealed class PetSession : IDisposable
{
    private AgentApiClient? _client;
    private CancellationTokenSource? _sseCts;
    private readonly object _gate = new();
    private readonly SseBubbleAggregator _bubbleAgg = new();

    public ChatHistory History { get; } = new();
    public AttachmentStaging Attachments { get; } = new();

    public bool IsConnected
    {
        get { lock (_gate) return _client != null; }
    }

    public bool HasPendingAttachments => Attachments.HasPending;

    public void ClearHistory()
    {
        History.Clear();
        HistoryUpdated?.Invoke();
    }

    public event Action? HistoryUpdated;
    public event Action<string>? StatusChanged;
    public event Action<PetMood>? MoodChanged;
    public event Action<bool, string>? ConnectionChanged;
    public event Action<string>? ReplyCompleted;
    public event Action<string>? SseUnstable;

    private volatile bool _expectingReply;
    private volatile bool _sseHealthy = true;

    public async Task ConnectAsync(AppSettings settings, string password, CancellationToken ct = default)
    {
        await DisconnectAsync(notify: false).ConfigureAwait(false);

        var deviceId = settings.DeviceId;
        if (string.IsNullOrWhiteSpace(deviceId))
            deviceId = "pc-" + Environment.MachineName;

        var client = new AgentApiClient(settings.BaseUrl, deviceId);
        await client.PingAsync(ct).ConfigureAwait(false);
        await client.LoginAsync(settings.Username, password, ct).ConfigureAwait(false);

        lock (_gate) { _client = client; }

        settings.DeviceId = deviceId;
        settings.Save();
        StartSse();
        _sseHealthy = true;
        _expectingReply = false;
        StatusChanged?.Invoke("已连接 " + settings.BaseUrl);
        ConnectionChanged?.Invoke(true, "已连接 " + settings.BaseUrl);
        MoodChanged?.Invoke(PetMood.Happy);
        DebugLog.Info("api", "connected");
    }

    public async Task DisconnectAsync(bool notify = true)
    {
        StopSse();
        _expectingReply = false;
        _sseHealthy = true;
        AgentApiClient? c;
        lock (_gate) { c = _client; _client = null; }
        var hadClient = c != null;
        if (c != null)
        {
            try { await c.LogoutAsync().ConfigureAwait(false); } catch { /* */ }
            c.Dispose();
        }

        if (notify && hadClient)
        {
            StatusChanged?.Invoke("已断开");
            ConnectionChanged?.Invoke(false, "已断开 · 请打开设置重新连接");
            MoodChanged?.Invoke(PetMood.Confused);
        }
    }

    public async Task UploadPathsAsync(IEnumerable<string> paths, CancellationToken ct = default)
    {
        AgentApiClient? c;
        lock (_gate) { c = _client; }
        if (c == null) throw new InvalidOperationException("请先在设置中登录 AgentTest");

        var entries = FileUploadHelper.CollectUploadEntries(paths).ToList();
        if (entries.Count == 0) throw new InvalidOperationException("未找到可上传的文件");

        MoodChanged?.Invoke(PetMood.Uploading);
        try
        {
            var result = await c.UploadAsync(Attachments.StagingId, entries, ct).ConfigureAwait(false);

            var pending = new List<PendingAttachment>();
            foreach (var (entryName, fullPath) in entries)
            {
                var server = result.Files.FirstOrDefault(f =>
                    string.Equals(f.Name, entryName, StringComparison.OrdinalIgnoreCase) ||
                    f.Name.Replace('\\', '/').EndsWith(entryName.Replace('\\', '/'), StringComparison.OrdinalIgnoreCase));
                pending.Add(new PendingAttachment
                {
                    DisplayName = server?.Name ?? entryName,
                    Kind = server?.Kind ?? FileUploadHelper.GuessKind(entryName),
                    LocalPath = fullPath
                });
            }

            if (Attachments.Items.Count > 0 &&
                !string.IsNullOrWhiteSpace(Attachments.StagingId) &&
                string.Equals(Attachments.StagingId, result.StagingId, StringComparison.Ordinal))
            {
                Attachments.AppendUploaded(result.StagingId, pending);
            }
            else
            {
                Attachments.SetUploaded(result.StagingId, pending);
            }

            if (result.Skipped.Count > 0)
                History.AddSystem("已跳过: " + string.Join("; ", result.Skipped));
            HistoryUpdated?.Invoke();
            MoodChanged?.Invoke(PetMood.Idle);
        }
        catch
        {
            MoodChanged?.Invoke(PetMood.Sad);
            throw;
        }
    }

    public async Task RemoveAttachmentAsync(string attachmentId, CancellationToken ct = default)
    {
        var item = Attachments.Items.FirstOrDefault(x => x.Id == attachmentId);
        if (item == null) return;

        Attachments.Items.Remove(item);
        if (Attachments.Items.Count == 0)
        {
            Attachments.Clear();
            HistoryUpdated?.Invoke();
            return;
        }

        var paths = Attachments.Items
            .Select(x => x.LocalPath)
            .Where(p => !string.IsNullOrWhiteSpace(p))
            .ToList();
        if (paths.Count != Attachments.Items.Count)
        {
            History.AddSystem("无法移除：缺少本地文件引用，已清空附件");
            Attachments.Clear();
            HistoryUpdated?.Invoke();
            return;
        }

        Attachments.Clear();
        HistoryUpdated?.Invoke();
        await UploadPathsAsync(paths, ct).ConfigureAwait(false);
    }

    public async Task<string> SendAsync(string message, CancellationToken ct = default)
    {
        AgentApiClient? c;
        lock (_gate) { c = _client; }
        if (c == null) throw new InvalidOperationException("请先在设置中登录 AgentTest");

        message = message.Trim();
        if (message.Length == 0 && !Attachments.HasPending)
            throw new InvalidOperationException("请输入消息或添加附件");

        if (message.Length > 0)
            History.AddUser(message);
        else
            History.AddUser("[附件]");
        HistoryUpdated?.Invoke();

        _bubbleAgg.BeginTurn();
        _expectingReply = true;
        MoodChanged?.Invoke(PetMood.Working);

        var stagingId = Attachments.StagingId;
        try
        {
            var turnId = await c.ChatAsync(message, stagingId, ct).ConfigureAwait(false);
            Attachments.Clear();
            HistoryUpdated?.Invoke();
            if (!string.IsNullOrEmpty(turnId))
                StatusChanged?.Invoke("回合: " + turnId);
            return turnId;
        }
        catch
        {
            _expectingReply = false;
            Attachments.Clear();
            HistoryUpdated?.Invoke();
            MoodChanged?.Invoke(PetMood.Sad);
            throw;
        }
    }

    private void StartSse()
    {
        StopSse();
        _sseCts = new CancellationTokenSource();
        var ct = _sseCts.Token;
        _ = Task.Run(async () =>
        {
            while (!ct.IsCancellationRequested)
            {
                AgentApiClient? c;
                lock (_gate) { c = _client; }
                if (c == null) break;
                try
                {
                    await c.StreamEventsAsync(ev =>
                    {
                        if (!_sseHealthy)
                        {
                            _sseHealthy = true;
                            StatusChanged?.Invoke("已连接");
                            ConnectionChanged?.Invoke(true, "SSE 已恢复");
                        }

                        if (!_bubbleAgg.TryUpdate(ev, out var src, out var text, out var isFinal))
                            return;
                        var key = string.IsNullOrWhiteSpace(ev.MessageId) ? null : ev.MessageId.Trim();
                        History.UpsertAssistant(src, text, isFinal, key);
                        HistoryUpdated?.Invoke();
                        MoodChanged?.Invoke(isFinal ? PetMood.Happy : PetMood.Working);

                        if (isFinal && _expectingReply)
                        {
                            _expectingReply = false;
                            var preview = MarkdownHelper.PreviewPlain(src, text, 80);
                            if (string.IsNullOrWhiteSpace(preview))
                                preview = "回复已完成";
                            ReplyCompleted?.Invoke(preview);
                        }
                    }, ct).ConfigureAwait(false);
                }
                catch (OperationCanceledException) { break; }
                catch (Exception ex)
                {
                    DebugLog.Warn("sse", ex.Message);
                    if (_sseHealthy)
                    {
                        _sseHealthy = false;
                        var msg = "SSE 断开，正在重连…";
                        History.AddSystem(msg + " (" + ex.Message + ")");
                        HistoryUpdated?.Invoke();
                        SseUnstable?.Invoke(msg);
                        MoodChanged?.Invoke(PetMood.Confused);
                    }
                    await Task.Delay(2000, ct).ConfigureAwait(false);
                }
            }
        }, ct);
    }

    private void StopSse()
    {
        _sseCts?.Cancel();
        _sseCts?.Dispose();
        _sseCts = null;
    }

    public void Shutdown()
    {
        StopSse();
        AgentApiClient? c;
        lock (_gate) { c = _client; _client = null; }
        if (c == null) return;
        try { c.Dispose(); } catch { /* */ }
    }

    public void Dispose() => Shutdown();
}

public enum PetMood
{
    Idle,
    Working,
    Resting,
    Happy,
    Sad,
    Confused,
    Uploading
}
