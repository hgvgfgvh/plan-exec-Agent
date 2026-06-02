using System.Diagnostics;
using System.Net.Http;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace AgentTestCat.Services;

/// <summary>内核就绪后：申请 WebUI 一次性 handoff，并用系统默认浏览器打开已登录页面。</summary>
public static class WebUiBrowserLauncher
{
    private sealed class HandoffResponse
    {
        [JsonPropertyName("handoff")]
        public string? Handoff { get; set; }
    }

    public static string ResolveWebPassword(AppSettings settings, string yamlPath)
    {
        if (!string.IsNullOrWhiteSpace(settings.Password))
            return settings.Password.Trim();
        try
        {
            return AppYamlConfigurator.ReadWebAuth(yamlPath).Password;
        }
        catch
        {
            return "";
        }
    }

    public static async Task TryOpenLoggedInBrowserAsync(
        AppSettings settings,
        string yamlPath,
        CancellationToken ct = default)
    {
        if (!settings.OpenWebUiOnStartup)
        {
            DebugLog.Info("webui", "skip browser: OpenWebUiOnStartup=false");
            return;
        }

        var password = ResolveWebPassword(settings, yamlPath);
        if (string.IsNullOrWhiteSpace(password))
        {
            DebugLog.Warn("webui", "skip browser: web.password empty in settings and app.yaml");
            OpenBrowserFallback(settings.BaseUrl);
            return;
        }

        var baseUrl = (settings.BaseUrl ?? AgentInstallPaths.DefaultWebUrl).TrimEnd('/');
        await Task.Delay(600, ct).ConfigureAwait(false);

        try
        {
            using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(20) };
            var body = new
            {
                username = string.IsNullOrWhiteSpace(settings.Username) ? "admin" : settings.Username.Trim(),
                password,
                channel = "web",
                device_id = "browser-" + Environment.MachineName,
                session_id = "browser-" + Guid.NewGuid().ToString("N"),
            };

            using var res = await http.PostAsJsonAsync($"{baseUrl}/api/session-handoff", body, ct)
                .ConfigureAwait(false);

            if (res.IsSuccessStatusCode)
            {
                var payload = await res.Content.ReadFromJsonAsync<HandoffResponse>(cancellationToken: ct)
                    .ConfigureAwait(false);
                var handoff = payload?.Handoff?.Trim();
                if (!string.IsNullOrEmpty(handoff))
                {
                    var url = $"{baseUrl}/auth/handoff?h={Uri.EscapeDataString(handoff)}";
                    DebugLog.Info("webui", "open handoff url");
                    LaunchBrowser(url);
                    return;
                }
            }

            var bodyText = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            DebugLog.Warn("webui", $"session-handoff {(int)res.StatusCode}: {bodyText.Trim()}");
        }
        catch (Exception ex)
        {
            DebugLog.Warn("webui", "session-handoff error: " + ex.Message);
        }

        DebugLog.Info("webui", "fallback open login page");
        OpenBrowserFallback(baseUrl);
    }

    public static void OpenBrowserFallback(string? baseUrl)
    {
        var url = (baseUrl ?? AgentInstallPaths.DefaultWebUrl).TrimEnd('/') + "/";
        LaunchBrowser(url);
    }

    private static void LaunchBrowser(string url)
    {
        try
        {
            Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
        }
        catch (Exception ex)
        {
            DebugLog.Warn("webui", "Process.Start failed: " + ex.Message);
        }
    }
}
