using System.IO;

namespace AgentTestCat.Services;

/// <summary>定位 AgentTest 安装根目录（开发仓库或解压后的发布包）。所有配置与数据均相对该目录。</summary>
public static class AgentInstallPaths
{
    public const string DefaultWebUrl = "http://127.0.0.1:8765";

    /// <summary>查找安装根：含 config/ 且存在 app.yaml 或（app.example.yaml + AgentTest.exe）。</summary>
    public static string? FindInstallRoot()
    {
        var env = Environment.GetEnvironmentVariable("AGENTTEST_ROOT");
        if (!string.IsNullOrWhiteSpace(env))
        {
            var p = Path.GetFullPath(env.Trim());
            if (IsInstallRoot(p)) return p;
        }

        foreach (var start in CandidateStartDirs())
        {
            var dir = new DirectoryInfo(start);
            for (var i = 0; i < 12 && dir != null; i++, dir = dir.Parent)
            {
                if (IsInstallRoot(dir.FullName))
                    return dir.FullName;
            }
        }

        return null;
    }

    public static string ConfigYamlPath(string root) =>
        Path.Combine(root, "config", "app.yaml");

    public static string ConfigExampleYamlPath(string root) =>
        Path.Combine(root, "config", "app.example.yaml");

    /// <summary>首次运行：从 app.example.yaml 生成 config/app.yaml（不覆盖已有文件）。</summary>
    public static void EnsureConfigFromExample(string root)
    {
        var yaml = ConfigYamlPath(root);
        if (File.Exists(yaml)) return;

        var example = ConfigExampleYamlPath(root);
        if (!File.Exists(example))
            throw new FileNotFoundException("未找到配置模板 config/app.example.yaml", example);

        Directory.CreateDirectory(Path.GetDirectoryName(yaml)!);
        File.Copy(example, yaml);
    }

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

    private static bool IsInstallRoot(string root)
    {
        if (!Directory.Exists(Path.Combine(root, "config")))
            return false;

        if (File.Exists(Path.Combine(root, "config", "app.yaml")))
            return true;

        return File.Exists(Path.Combine(root, "config", "app.example.yaml"))
               && File.Exists(Path.Combine(root, "AgentTest.exe"));
    }
}
