using System.IO;
using System.Text.Json;

namespace AgentTestCat.Services;

public sealed class AppSettings
{
    public string BaseUrl { get; set; } = "http://127.0.0.1:8765";
    public string Username { get; set; } = "admin";
    public string Password { get; set; } = "";
    public bool RememberPassword { get; set; }
    public string DeviceId { get; set; } = "";
    public bool Debug { get; set; }
    public double WindowLeft { get; set; } = 200;
    public double WindowTop { get; set; } = 200;
    public double ExpandedPanelWidth { get; set; } = 500;
    public double ExpandedPanelHeight { get; set; } = 420;
    public double ExpandedPanelOffsetX { get; set; } = 8;
    public double ExpandedPanelOffsetY { get; set; } = 8;
    public bool EnableSound { get; set; } = true;
    public bool EnableReplyNotify { get; set; } = true;
    /// <summary>启动内核成功后用默认浏览器打开 WebUI（经 handoff 自动登录）。</summary>
    public bool OpenWebUiOnStartup { get; set; } = true;

    public static string ConfigDir =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "AgentTestPCAPPCat");

    public static string ConfigPath => Path.Combine(ConfigDir, "settings.json");

    public static AppSettings Load()
    {
        try
        {
            if (!File.Exists(ConfigPath))
                return new AppSettings();
            var json = File.ReadAllText(ConfigPath);
            var s = JsonSerializer.Deserialize<AppSettings>(json) ?? new AppSettings();
            s.BaseUrl = (s.BaseUrl ?? "").TrimEnd('/');
            if (string.IsNullOrWhiteSpace(s.BaseUrl))
                s.BaseUrl = "http://127.0.0.1:8765";
            return s;
        }
        catch
        {
            return new AppSettings();
        }
    }

    public void Save()
    {
        Directory.CreateDirectory(ConfigDir);
        var json = JsonSerializer.Serialize(this, new JsonSerializerOptions { WriteIndented = true });
        File.WriteAllText(ConfigPath, json);
    }
}
