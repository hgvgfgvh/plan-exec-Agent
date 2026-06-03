using System.IO;
using System.Net;
using System.Net.Http;
using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace AgentTestCat.Services;

public sealed class OutputEvent
{
    [JsonPropertyName("source")]
    public string Source { get; set; } = "";

    [JsonPropertyName("text")]
    public string Text { get; set; } = "";

    [JsonPropertyName("turn_id")]
    public string? TurnId { get; set; }

    [JsonPropertyName("message_id")]
    public string? MessageId { get; set; }

    [JsonPropertyName("event")]
    public string? Event { get; set; }
}

public sealed class AgentApiClient : IDisposable
{
    public const string ChannelDesktop = "desktop";

    private readonly HttpClient _http;
    private readonly CookieContainer _cookies = new();
    private readonly string _sessionId = $"desktop-{Guid.NewGuid():N}";

    public string BaseUrl { get; private set; }
    public string DeviceId { get; set; }

    public AgentApiClient(string baseUrl, string deviceId)
    {
        BaseUrl = baseUrl.TrimEnd('/');
        DeviceId = deviceId;
        var handler = new HttpClientHandler { CookieContainer = _cookies, UseCookies = true };
        _http = new HttpClient(handler) { Timeout = TimeSpan.FromMinutes(30) };
    }

    public void UpdateBaseUrl(string baseUrl) => BaseUrl = baseUrl.TrimEnd('/');

    public async Task PingAsync(CancellationToken ct = default)
    {
        using var res = await _http.GetAsync($"{BaseUrl}/", ct).ConfigureAwait(false);
        if ((int)res.StatusCode >= 500)
            throw new HttpRequestException($"Server {res.StatusCode}");
    }

    public async Task LoginAsync(string username, string password, CancellationToken ct = default)
    {
        var body = new
        {
            username,
            password,
            channel = ChannelDesktop,
            device_id = DeviceId,
            session_id = _sessionId
        };
        using var res = await _http.PostAsJsonAsync($"{BaseUrl}/api/login", body, ct).ConfigureAwait(false);
        if (res.StatusCode == HttpStatusCode.Unauthorized)
            throw new HttpRequestException("用户名或密码错误（默认账号 admin，密码见 AgentTest WebUI 配置）");
        await EnsureOk(res, ct).ConfigureAwait(false);

        var uri = new Uri(BaseUrl + "/");
        var cookies = _cookies.GetCookies(uri);
        if (cookies["at_sess"] == null)
            throw new HttpRequestException("登录成功但未收到会话 Cookie，请检查服务地址是否正确");
        DebugLog.Info("api", $"login ok user={username}");
    }

    public async Task LogoutAsync(CancellationToken ct = default)
    {
        try
        {
            using var res = await _http.PostAsync($"{BaseUrl}/api/logout", null, ct).ConfigureAwait(false);
        }
        catch { /* ignore */ }
    }

    public async Task<string> ChatAsync(string message, string? stagingId = null, CancellationToken ct = default)
    {
        var payload = new Dictionary<string, object?>
        {
            ["message"] = message,
            ["channel"] = ChannelDesktop,
            ["device_id"] = DeviceId,
            ["session_id"] = _sessionId,
        };
        if (!string.IsNullOrWhiteSpace(stagingId))
            payload["staging_id"] = stagingId;

        using var res = await _http.PostAsJsonAsync($"{BaseUrl}/api/chat", payload, ct).ConfigureAwait(false);
        var raw = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        if (!res.IsSuccessStatusCode)
            throw new HttpRequestException($"chat {res.StatusCode}: {raw}");

        using var doc = JsonDocument.Parse(raw);
        if (doc.RootElement.TryGetProperty("ok", out var ok) && !ok.GetBoolean())
        {
            var err = doc.RootElement.TryGetProperty("error", out var e) ? e.GetString() : "chat failed";
            throw new HttpRequestException(err ?? "chat failed");
        }
        if (doc.RootElement.TryGetProperty("turn_id", out var tid))
            return tid.GetString() ?? "";
        return "";
    }

    public async Task<UploadResult> UploadAsync(string? stagingId, IEnumerable<(string EntryName, string FullPath)> files, CancellationToken ct = default)
    {
        var list = files.ToList();
        if (list.Count == 0)
            throw new ArgumentException("没有可上传的文件");

        using var content = new MultipartFormDataContent();
        if (!string.IsNullOrWhiteSpace(stagingId))
            content.Add(new StringContent(stagingId), "staging_id");

        foreach (var (entryName, fullPath) in list)
        {
            var stream = File.OpenRead(fullPath);
            var part = new StreamContent(stream);
            part.Headers.ContentType = new System.Net.Http.Headers.MediaTypeHeaderValue("application/octet-stream");
            content.Add(part, "files", entryName.Replace('\\', '/'));
        }

        using var res = await _http.PostAsync($"{BaseUrl}/api/upload", content, ct).ConfigureAwait(false);
        var raw = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        if (!res.IsSuccessStatusCode)
            throw new HttpRequestException($"upload {res.StatusCode}: {raw}");

        using var doc = JsonDocument.Parse(raw);
        var result = new UploadResult();
        if (doc.RootElement.TryGetProperty("staging_id", out var sid))
            result.StagingId = sid.GetString() ?? "";
        if (doc.RootElement.TryGetProperty("files", out var filesEl) && filesEl.ValueKind == JsonValueKind.Array)
        {
            foreach (var item in filesEl.EnumerateArray())
            {
                var name = item.TryGetProperty("name", out var n) ? n.GetString() ?? "" : "";
                var kind = item.TryGetProperty("kind", out var k) ? k.GetString() : null;
                if (!string.IsNullOrEmpty(name))
                    result.Files.Add(new UploadFileEntry { Name = name, Kind = kind });
            }
        }
        if (doc.RootElement.TryGetProperty("skipped", out var skippedEl) && skippedEl.ValueKind == JsonValueKind.Array)
        {
            foreach (var item in skippedEl.EnumerateArray())
            {
                var s = item.GetString();
                if (!string.IsNullOrEmpty(s))
                    result.Skipped.Add(s);
            }
        }
        DebugLog.Info("api", $"upload ok staging={result.StagingId} files={result.Files.Count}");
        return result;
    }

    public async Task StreamEventsAsync(Action<OutputEvent> onEvent, CancellationToken ct)
    {
        var q = $"channel={Uri.EscapeDataString(ChannelDesktop)}&device_id={Uri.EscapeDataString(DeviceId)}&session_id={Uri.EscapeDataString(_sessionId)}";
        using var req = new HttpRequestMessage(HttpMethod.Get, $"{BaseUrl}/api/events?{q}");
        req.Headers.TryAddWithoutValidation("Accept", "text/event-stream");
        using var res = await _http.SendAsync(req, HttpCompletionOption.ResponseHeadersRead, ct).ConfigureAwait(false);
        await EnsureOk(res, ct).ConfigureAwait(false);

        await using var stream = await res.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        using var reader = new StreamReader(stream, Encoding.UTF8);
        DebugLog.Info("sse", "stream start");

        while (!reader.EndOfStream)
        {
            ct.ThrowIfCancellationRequested();
            var line = await reader.ReadLineAsync(ct).ConfigureAwait(false);
            if (line == null) break;
            if (!line.StartsWith("data: ", StringComparison.Ordinal)) continue;
            var payload = line["data: ".Length..].Trim();
            if (string.IsNullOrEmpty(payload)) continue;

            OutputEvent? ev = null;
            try
            {
                ev = JsonSerializer.Deserialize<OutputEvent>(payload);
            }
            catch
            {
                ev = new OutputEvent { Source = "raw", Text = payload };
            }
            if (ev == null) continue;

            var isStream = IsStreamControlEvent(ev);
            if (!isStream && string.IsNullOrWhiteSpace(ev.Text))
                continue;

            if (!string.IsNullOrWhiteSpace(ev.Text))
            {
                var preview = ev.Text.Length > 80 ? ev.Text[..80] + "…" : ev.Text;
                DebugLog.Debug("sse", $"source={ev.Source} event={ev.Event} preview={preview}");
            }
            else
            {
                DebugLog.Debug("sse", $"source={ev.Source} event={ev.Event} (stream control)");
            }

            onEvent(ev);
        }
        DebugLog.Debug("sse", "stream end");
    }

    private static bool IsStreamControlEvent(OutputEvent ev)
    {
        var msgId = (ev.MessageId ?? "").Trim();
        if (string.IsNullOrEmpty(msgId)) return false;
        var eventType = (ev.Event ?? "").Trim();
        return eventType is "delta" or "final";
    }

    private static async Task EnsureOk(HttpResponseMessage res, CancellationToken ct)
    {
        if (res.IsSuccessStatusCode) return;
        var body = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        throw new HttpRequestException($"{res.StatusCode}: {body}");
    }

    public void Dispose() => _http.Dispose();
}
