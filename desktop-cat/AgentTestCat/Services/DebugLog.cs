using System.IO;

namespace AgentTestCat.Services;

public static class DebugLog
{
    private static readonly object Gate = new();
    private static StreamWriter? _writer;
    private static bool _enabled;

    public static string? Path { get; private set; }

    public static void Init(bool settingsDebug)
    {
        var env = Environment.GetEnvironmentVariable("AGENTTEST_CAT_DEBUG");
        _enabled = settingsDebug || env is "1" or "true" or "yes" or "on";
        if (!_enabled) return;

        var dir = AppSettings.ConfigDir;
        Directory.CreateDirectory(dir);
        Path = System.IO.Path.Combine(dir, "debug.log");
        _writer = new StreamWriter(Path, append: true) { AutoFlush = true };
        Info("boot", "debug enabled");
    }

    public static void Info(string tag, string message) => Write("INFO", tag, message);
    public static void Debug(string tag, string message) => Write("DEBUG", tag, message);
    public static void Warn(string tag, string message) => Write("WARN", tag, message);
    public static void Error(string tag, string message) => Write("ERROR", tag, message);

    private static void Write(string level, string tag, string message)
    {
        if (!_enabled) return;
        var line = $"{DateTime.Now:yyyy-MM-dd HH:mm:ss.fff} [{level}][{tag}] {message}";
        lock (Gate)
        {
            _writer?.WriteLine(line);
        }
        System.Diagnostics.Debug.WriteLine(line);
    }

    public static void Close()
    {
        lock (Gate)
        {
            _writer?.Dispose();
            _writer = null;
        }
    }
}
