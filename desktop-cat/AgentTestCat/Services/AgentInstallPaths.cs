using System.IO;

namespace AgentTestCat.Services;

/// <summary>定位 AgentTest 工程根目录与 AgentTest.exe。</summary>
public static class AgentInstallPaths
{
    public const string DefaultWebUrl = "http://127.0.0.1:8765";

    public static string? FindProjectRoot()
    {
        var env = Environment.GetEnvironmentVariable("AGENTTEST_ROOT");
        if (!string.IsNullOrWhiteSpace(env))
        {
            var p = Path.GetFullPath(env.Trim());
            if (HasAppYaml(p)) return p;
        }

        foreach (var start in CandidateStartDirs())
        {
            var dir = new DirectoryInfo(start);
            for (var i = 0; i < 12 && dir != null; i++, dir = dir.Parent)
            {
                if (HasAppYaml(dir.FullName))
                    return dir.FullName;
            }
        }

        return null;
    }

    public static string ConfigYamlPath(string root) =>
        Path.Combine(root, "config", "app.yaml");

    public static string? FindAgentTestExe(string root)
    {
        var candidates = new[]
        {
            Path.Combine(root, "AgentTest.exe"),
            Path.Combine(root, "bin", "AgentTest.exe"),
            Path.Combine(root, "dist", "AgentTest.exe"),
        };
        foreach (var c in candidates)
        {
            if (File.Exists(c)) return c;
        }
        return null;
    }

    private static IEnumerable<string> CandidateStartDirs()
    {
        yield return AppContext.BaseDirectory.TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        if (!string.IsNullOrEmpty(Environment.CurrentDirectory))
            yield return Path.GetFullPath(Environment.CurrentDirectory);
        var proc = Environment.ProcessPath;
        if (!string.IsNullOrEmpty(proc))
            yield return Path.GetDirectoryName(proc)!;
    }

    private static bool HasAppYaml(string root) =>
        File.Exists(Path.Combine(root, "config", "app.yaml"));
}
