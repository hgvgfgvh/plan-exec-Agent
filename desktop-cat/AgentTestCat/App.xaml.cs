using System.Windows;
using AgentTestCat.Services;
using AgentTestCat.Views;
using Forms = System.Windows.Forms;

namespace AgentTestCat;

public partial class App : System.Windows.Application
{
    private AppSettings _settings = null!;
    private PetSession _session = null!;
    private CatSoundService? _sounds;
    private TrayNotifier? _trayNotifier;
    private PetWindow? _petWindow;
    private Forms.NotifyIcon? _tray;
    private AgentBackendHost? _backend;
    private bool _shuttingDown;
    private bool _sessionShutdown;
    private string _projectRoot = "";
    private string _yamlPath = "";

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        ShutdownMode = ShutdownMode.OnExplicitShutdown;
        _ = StartupAsync();
    }

    private async Task StartupAsync()
    {
        _settings = AppSettings.Load();
        if (string.IsNullOrWhiteSpace(_settings.DeviceId))
            _settings.DeviceId = "pc-" + Environment.MachineName;

        DebugLog.Init(_settings.Debug);
        DebugLog.Info("boot", "AgentTest Cat WPF starting (integrated)");

        try
        {
            var root = AgentInstallPaths.FindProjectRoot();
            if (root == null)
            {
                System.Windows.MessageBox.Show(
                    "未找到 AgentTest 工程根目录（需要存在 config/app.yaml）。\n\n" +
                    "请从 AgentTest 仓库根目录启动小猫，或设置环境变量 AGENTTEST_ROOT。",
                    "AgentTest 小猫",
                    MessageBoxButton.OK,
                    MessageBoxImage.Error);
                Shutdown();
                return;
            }

            _projectRoot = root;
            _yamlPath = AgentInstallPaths.ConfigYamlPath(root);

            if (!AppYamlConfigurator.HasDeepSeekKey(_yamlPath))
            {
                var wizard = new SetupWizardWindow(root, _yamlPath);
                if (wizard.ShowDialog() != true)
                {
                    Shutdown();
                    return;
                }

                var web = AppYamlConfigurator.ReadWebAuth(_yamlPath);
                _settings.Username = web.Username;
                if (!string.IsNullOrEmpty(wizard.WebPassword))
                    _settings.Password = wizard.WebPassword;
                else if (!string.IsNullOrEmpty(web.Password))
                    _settings.Password = web.Password;
                _settings.RememberPassword = true;
                _settings.Save();
            }

            SyncWebCredentialsFromYaml();
            AppYamlConfigurator.NormalizeFileFormat(_yamlPath);

            _backend = new AgentBackendHost();
            await _backend.EnsureRunningAsync(
                _projectRoot,
                _yamlPath,
                _settings.BaseUrl,
                CancellationToken.None).ConfigureAwait(true);

            await WebUiBrowserLauncher.TryOpenLoggedInBrowserAsync(_settings, _yamlPath)
                .ConfigureAwait(true);

            _session = new PetSession();
            _sounds = new CatSoundService(_settings, Dispatcher);

            _petWindow = new PetWindow(_settings, _session, _sounds, OpenSettings, ShutdownApp);
            _petWindow.Closing += (_, args) =>
            {
                if (!_shuttingDown)
                {
                    args.Cancel = true;
                    _petWindow.Hide();
                }
            };
            _petWindow.Show();

            SetupTray();
            _trayNotifier = new TrayNotifier(_settings, _tray!, _session, _sounds, () => _petWindow);

            await TryAutoConnectAsync().ConfigureAwait(true);

            DebugLog.Info("boot", "ready");
        }
        catch (Exception ex)
        {
            DebugLog.Error("boot", ex.ToString());
            System.Windows.MessageBox.Show(
                "启动失败：\n" + ex.Message,
                "AgentTest 小猫",
                MessageBoxButton.OK,
                MessageBoxImage.Error);
            Shutdown();
        }
    }

    private void SyncWebCredentialsFromYaml()
    {
        try
        {
            var web = AppYamlConfigurator.ReadWebAuth(_yamlPath);
            _settings.Username = web.Username;
            _settings.BaseUrl = AgentInstallPaths.DefaultWebUrl;
            if (string.IsNullOrEmpty(_settings.Password) && !string.IsNullOrEmpty(web.Password))
                _settings.Password = web.Password;
        }
        catch (Exception ex)
        {
            DebugLog.Warn("boot", "read web auth: " + ex.Message);
        }
    }

    private async Task TryAutoConnectAsync()
    {
        if (string.IsNullOrEmpty(_settings.Password))
        {
            _petWindow?.UpdateConnectionStatus(false, "未连接 · 请在设置中填写 WebUI 密码");
            return;
        }

        try
        {
            await _session.ConnectAsync(_settings, _settings.Password).ConfigureAwait(true);
            _petWindow?.UpdateConnectionStatus(true, "已连接 · " + _settings.BaseUrl);
        }
        catch (Exception ex)
        {
            DebugLog.Warn("api", "autoconnect: " + ex.Message);
            _petWindow?.UpdateConnectionStatus(false, "未连接 · " + ex.Message);
        }
    }

    private void OpenSettings()
    {
        var w = new SettingsWindow(_settings, _session, _projectRoot, _yamlPath) { Owner = _petWindow };
        w.ShowDialog();
    }

    private void SetupTray()
    {
        _tray = new Forms.NotifyIcon
        {
            Text = "AgentTest Cat · 启动中",
            Visible = true,
            Icon = CatIconHelper.GetTrayIcon(),
        };

        var menu = new Forms.ContextMenuStrip();
        menu.Items.Add("显示小猫", null, (_, _) =>
        {
            _petWindow?.Show();
            _petWindow?.Activate();
        });
        menu.Items.Add("API Key 配置…", null, (_, _) => OpenSetupWizard());
        menu.Items.Add("打开 WebUI", null, (_, _) => _ = OpenWebUiAsync());
        menu.Items.Add("连接设置…", null, (_, _) => OpenSettings());
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add("退出", null, (_, _) => ShutdownApp());

        _tray.ContextMenuStrip = menu;
        _tray.DoubleClick += (_, _) =>
        {
            _petWindow?.Show();
            _petWindow?.Activate();
        };
    }

    private async Task OpenWebUiAsync()
    {
        await WebUiBrowserLauncher.TryOpenLoggedInBrowserAsync(_settings, _yamlPath).ConfigureAwait(true);
    }

    private void OpenSetupWizard()
    {
        if (string.IsNullOrEmpty(_projectRoot)) return;
        var wizard = new SetupWizardWindow(_projectRoot, _yamlPath) { Owner = _petWindow };
        if (wizard.ShowDialog() != true) return;
        SyncWebCredentialsFromYaml();
        if (!string.IsNullOrEmpty(wizard.WebPassword))
        {
            _settings.Password = wizard.WebPassword;
            _settings.RememberPassword = true;
            _settings.Save();
        }
        System.Windows.MessageBox.Show(
            _petWindow,
            "配置已保存。若内核已在运行，部分 Key 需重启 AgentTest 后生效。\n请退出小猫后重新启动，或手动结束 AgentTest.exe 再开。",
            "提示",
            MessageBoxButton.OK,
            MessageBoxImage.Information);
    }

    private void ShutdownApp()
    {
        if (_shuttingDown) return;
        _shuttingDown = true;
        DebugLog.Info("boot", "shutdown");

        _petWindow?.PrepareForShutdown();
        if (_petWindow != null)
        {
            _settings.WindowLeft = _petWindow.Left;
            _settings.WindowTop = _petWindow.Top;
            _settings.Save();
        }

        _tray?.Dispose();
        _tray = null;

        if (!_sessionShutdown)
        {
            _sessionShutdown = true;
            _session?.Shutdown();
        }

        _backend?.Dispose();
        _backend = null;

        DebugLog.Close();
        Current.Shutdown();
    }

    protected override void OnExit(ExitEventArgs e)
    {
        _tray?.Dispose();
        if (!_sessionShutdown)
            _session?.Shutdown();
        _backend?.Dispose();
        DebugLog.Close();
        base.OnExit(e);
    }
}
