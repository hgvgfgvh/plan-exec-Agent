using System.Diagnostics;
using System.IO;
using System.Net.Http;

namespace AgentTestCat.Services;

/// <summary>检测、启动并停止本机 AgentTest 进程。</summary>
public sealed class AgentBackendHost : IDisposable
{
    private static readonly HttpClient PingClient = new() { Timeout = TimeSpan.FromSeconds(3) };

    private Process? _process;
    private bool _weStarted;

    public static async Task<bool> IsReachableAsync(string baseUrl, CancellationToken ct = default)
    {
        try
        {
            using var res = await PingClient.GetAsync(baseUrl.TrimEnd('/') + "/", ct).ConfigureAwait(false);
            return (int)res.StatusCode < 500;
        }
        catch
        {
            return false;
        }
    }

    public async Task EnsureRunningAsync(string projectRoot, string configYamlPath, string baseUrl, CancellationToken ct)
    {
        if (await IsReachableAsync(baseUrl, ct).ConfigureAwait(false))
        {
            DebugLog.Info("backend", "AgentTest already listening at " + baseUrl);
            return;
        }

        var exe = AgentInstallPaths.FindAgentTestExe(projectRoot);
        if (exe == null)
            throw new FileNotFoundException(
                "未找到 AgentTest.exe。请先在工程根目录执行：go build -o AgentTest.exe .",
                Path.Combine(projectRoot, "AgentTest.exe"));

        var relConfig = Path.GetRelativePath(projectRoot, configYamlPath);
        if (relConfig.StartsWith("..", StringComparison.Ordinal))
            relConfig = configYamlPath;

        var psi = new ProcessStartInfo
        {
            FileName = exe,
            WorkingDirectory = projectRoot,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        psi.Environment["AGENTTEST_CONFIG"] = relConfig.Replace('\\', '/');

        DebugLog.Info("backend", $"starting {exe} AGENTTEST_CONFIG={psi.Environment["AGENTTEST_CONFIG"]}");
        _process = Process.Start(psi) ?? throw new InvalidOperationException("无法启动 AgentTest 进程");
        _weStarted = true;

        var deadline = DateTime.UtcNow.AddMinutes(3);
        while (DateTime.UtcNow < deadline)
        {
            ct.ThrowIfCancellationRequested();
            if (_process.HasExited)
                throw new InvalidOperationException(
                    $"AgentTest 进程已退出（代码 {_process.ExitCode}）。请检查 config/app.yaml 与 WorkSpace/mcp_bundled。");
            if (await IsReachableAsync(baseUrl, ct).ConfigureAwait(false))
            {
                DebugLog.Info("backend", "AgentTest ready");
                return;
            }
            await Task.Delay(500, ct).ConfigureAwait(false);
        }

        throw new TimeoutException("等待 AgentTest WebUI 启动超时（" + baseUrl + "）");
    }

    public void Dispose()
    {
        if (!_weStarted || _process == null) return;
        try
        {
            if (!_process.HasExited)
            {
                _process.CloseMainWindow();
                if (!_process.WaitForExit(2000))
                    _process.Kill(entireProcessTree: true);
            }
        }
        catch
        {
            /* best effort */
        }
        finally
        {
            _process.Dispose();
            _process = null;
        }
    }
}
