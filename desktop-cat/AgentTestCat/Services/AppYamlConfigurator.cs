using System.Collections.Generic;
using System.IO;
using System.Text;
using System.Text.RegularExpressions;

namespace AgentTestCat.Services;

/// <summary>读写 config/app.yaml 中的 API Key 与 Web 登录信息（单行更新，避免破坏 YAML 结构）。</summary>
public static class AppYamlConfigurator
{
    public sealed record WebAuth(string Username, string Password);

    private static readonly string[] EnvKeysFromDeepSeek =
    {
        "MEMORY_MCP_LLM_API_KEY",
        "SOUL_MCP_LLM_API_KEY",
        "MCP_MANAGER_LLM_API_KEY",
    };

    public static bool HasDeepSeekKey(string yamlPath)
    {
        if (!File.Exists(yamlPath)) return false;
        var key = ReadScalarInSection(ReadAllText(yamlPath), "deepseek_legacy", "api_key");
        if (string.IsNullOrWhiteSpace(key)) return false;
        return !IsPlaceholderKey(key);
    }

    private static bool IsPlaceholderKey(string key)
    {
        var k = key.Trim();
        if (k.Length == 0) return true;
        if (k.StartsWith("your-", StringComparison.OrdinalIgnoreCase)) return true;
        if (k.Contains("your-", StringComparison.OrdinalIgnoreCase)
            && k.Contains("api-key", StringComparison.OrdinalIgnoreCase))
            return true;
        return false;
    }

    public static WebAuth ReadWebAuth(string yamlPath)
    {
        var text = ReadAllText(yamlPath);
        var user = ReadScalarInSection(text, "web", "username") ?? "admin";
        var pass = ReadScalarInSection(text, "web", "password") ?? "";
        return new WebAuth(user.Trim(), UnquoteYaml(pass.Trim()));
    }

    /// <summary>桌面发布包：强制开启 WebUI 与 MCP（避免 example 中 enabled:false 导致小猫永久等待）。</summary>
    public static void EnsureDesktopDefaults(string yamlPath)
    {
        if (!File.Exists(yamlPath)) return;
        var text = ReadAllText(yamlPath);
        text = Regex.Replace(
            text,
            @"(^web:\s*\r?\n(?:[ \t].*\r?\n)*?[ \t]*enabled:\s*)false\b",
            "$1true",
            RegexOptions.Multiline | RegexOptions.CultureInvariant);
        text = Regex.Replace(
            text,
            @"(^capabilities:\s*\r?\n(?:[ \t].*\r?\n)*?[ \t]*mcp:\s*\r?\n(?:[ \t].*\r?\n)*?[ \t]*enabled:\s*)false\b",
            "$1true",
            RegexOptions.Multiline | RegexOptions.CultureInvariant);
        WriteAllText(yamlPath, CollapseExcessiveBlankLines(text));
    }

    /// <summary>整理 app.yaml 空行（不修改取值）。启动时可调用。</summary>
    public static void NormalizeFileFormat(string yamlPath)
    {
        if (!File.Exists(yamlPath)) return;
        WriteAllText(yamlPath, CollapseExcessiveBlankLines(ReadAllText(yamlPath)));
    }

    public static void ApplyApiKeys(string yamlPath, string deepseekKey, string? dashscopeKey, string? webPassword = null)
    {
        if (!File.Exists(yamlPath))
            throw new FileNotFoundException("未找到配置文件", yamlPath);

        var deep = deepseekKey.Trim();
        if (string.IsNullOrEmpty(deep))
            throw new ArgumentException("DeepSeek API Key 不能为空", nameof(deepseekKey));

        var lines = CollapseToLines(ReadAllText(yamlPath));
        lines = UpdateLines(lines, "deepseek_legacy", "api_key", deep);
        lines = UpdateLines(lines, "dashscope", "api_key", (dashscopeKey ?? "").Trim());

        foreach (var envKey in EnvKeysFromDeepSeek)
            lines = UpdateEnvKey(lines, envKey, deep);

        lines = UpdateTopLevelKey(lines, "llm_api_key", deep, parentSection: "run_view");

        if (!string.IsNullOrWhiteSpace(webPassword))
            lines = UpdateTopLevelKey(lines, "password", webPassword.Trim(), parentSection: "web");

        WriteAllText(yamlPath, JoinLines(lines));
    }

    private static List<string> UpdateLines(List<string> lines, string section, string field, string value)
    {
        var inSection = false;
        var sectionIndent = 0;
        for (var i = 0; i < lines.Count; i++)
        {
            var line = lines[i];
            var trimmed = line.TrimStart();
            var indent = line.Length - trimmed.Length;

            if (Regex.IsMatch(trimmed, $"^{Regex.Escape(section)}:\\s*$"))
            {
                inSection = true;
                sectionIndent = indent;
                continue;
            }

            if (!inSection) continue;

            if (trimmed.Length > 0 && !trimmed.StartsWith('#') && indent <= sectionIndent)
            {
                inSection = false;
                continue;
            }

            if (!Regex.IsMatch(trimmed, $"^{Regex.Escape(field)}:\\s*")) continue;

            lines[i] = line[..indent] + field + ": " + FormatYamlScalar(value);
            if (TryConsumeOrphanValueLine(lines, i + 1))
                lines.RemoveAt(i + 1);
        }

        return lines;
    }

    private static List<string> UpdateEnvKey(List<string> lines, string key, string value)
    {
        for (var i = 0; i < lines.Count; i++)
        {
            var line = lines[i];
            var trimmed = line.TrimStart();
            if (!Regex.IsMatch(trimmed, $"^{Regex.Escape(key)}:\\s*")) continue;

            var indent = line.Length - trimmed.Length;
            lines[i] = line[..indent] + key + ": " + FormatYamlScalar(value);
            if (TryConsumeOrphanValueLine(lines, i + 1))
                lines.RemoveAt(i + 1);
        }
        return lines;
    }

    private static List<string> UpdateTopLevelKey(List<string> lines, string field, string value, string parentSection)
    {
        var inParent = false;
        var parentIndent = 0;
        for (var i = 0; i < lines.Count; i++)
        {
            var line = lines[i];
            var trimmed = line.TrimStart();
            var indent = line.Length - trimmed.Length;

            if (Regex.IsMatch(trimmed, $"^{Regex.Escape(parentSection)}:\\s*$"))
            {
                inParent = true;
                parentIndent = indent;
                continue;
            }

            if (!inParent) continue;

            if (trimmed.Length > 0 && !trimmed.StartsWith('#') && indent <= parentIndent)
            {
                inParent = false;
                continue;
            }

            if (!Regex.IsMatch(trimmed, $"^{Regex.Escape(field)}:\\s*")) continue;

            lines[i] = line[..indent] + field + ": " + FormatYamlScalar(value);
            if (TryConsumeOrphanValueLine(lines, i + 1))
                lines.RemoveAt(i + 1);
        }
        return lines;
    }

    /// <summary>下一行是否为被误拆出去的 key 值（无冒号、非注释）。</summary>
    private static bool TryConsumeOrphanValueLine(List<string> lines, int index)
    {
        if (index >= lines.Count) return false;
        var next = lines[index].TrimStart();
        if (next.Length == 0) return false;
        if (next.StartsWith('#')) return false;
        if (next.Contains(':')) return false;
        return true;
    }

    private static List<string> CollapseToLines(string text)
    {
        return new List<string>(CollapseExcessiveBlankLines(text).Replace("\r\n", "\n").Split('\n'));
    }

    private static string JoinLines(List<string> lines) =>
        CollapseExcessiveBlankLines(string.Join("\n", lines));

    private static string CollapseExcessiveBlankLines(string text)
    {
        var lines = text.Replace("\r\n", "\n").Split('\n');
        var outLines = new List<string>(lines.Length);
        for (var i = 0; i < lines.Length; i++)
        {
            if (lines[i].Length == 0)
            {
                var j = i + 1;
                while (j < lines.Length && lines[j].Length == 0) j++;
                if (j < lines.Length && IsYamlSectionStarter(lines[j]))
                {
                    if (outLines.Count > 0 && outLines[^1].Length != 0)
                        outLines.Add("");
                }
                continue;
            }
            outLines.Add(lines[i]);
        }
        return string.Join("\n", outLines).TrimEnd() + "\n";
    }

    private static bool IsYamlSectionStarter(string line)
    {
        if (string.IsNullOrWhiteSpace(line)) return false;
        if (line.StartsWith("# =====", StringComparison.Ordinal)) return true;
        if (Regex.IsMatch(line, @"^[a-z_][a-z0-9_]*:\s*$")) return true;
        if (Regex.IsMatch(line, @"^      - name:")) return true;
        if (Regex.IsMatch(line, @"^# [a-z_].*:\s") && !line.StartsWith("#   ", StringComparison.Ordinal)) return true;
        return false;
    }

    private static string? ReadScalarInSection(string text, string section, string field)
    {
        var lines = text.Replace("\r\n", "\n").Split('\n');
        var inSection = false;
        var sectionIndent = 0;

        for (var i = 0; i < lines.Length; i++)
        {
            var line = lines[i];
            var trimmed = line.TrimStart();
            var indent = line.Length - trimmed.Length;

            if (Regex.IsMatch(trimmed, $"^{Regex.Escape(section)}:\\s*$"))
            {
                inSection = true;
                sectionIndent = indent;
                continue;
            }

            if (!inSection) continue;

            if (trimmed.Length > 0 && !trimmed.StartsWith('#') && indent <= sectionIndent)
                break;

            var m = Regex.Match(trimmed, $"^{Regex.Escape(field)}:\\s*(.*)$");
            if (!m.Success) continue;

            var rest = UnquoteYaml(m.Groups[1].Value.Trim());
            if (rest.Length > 0) return rest;

            var j = i + 1;
            while (j < lines.Length && lines[j].Length == 0) j++;
            if (j < lines.Length)
            {
                var next = lines[j].TrimStart();
                if (!next.StartsWith('#') && !next.Contains(':'))
                    return UnquoteYaml(next);
            }
            return "";
        }

        return null;
    }

    private static string FormatYamlScalar(string value)
    {
        if (value.Length == 0) return "\"\"";
        if (Regex.IsMatch(value, @"[\s:#\[\]{}'""&*!?|>@`]") || value.StartsWith('-') || value.StartsWith("sk-"))
            return "\"" + value.Replace("\\", "\\\\").Replace("\"", "\\\"") + "\"";
        return value;
    }

    private static string UnquoteYaml(string raw)
    {
        if (raw.Length >= 2 && raw[0] == '"' && raw[^1] == '"')
            return raw[1..^1].Replace("\\\"", "\"").Replace("\\\\", "\\");
        return raw;
    }

    private static string ReadAllText(string path) => File.ReadAllText(path, Encoding.UTF8);

    private static void WriteAllText(string path, string text) =>
        File.WriteAllText(path, text, new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
}
