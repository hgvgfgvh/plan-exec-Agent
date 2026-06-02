using System.Windows;
using System.Windows.Media;
using AgentTestCat.Services;

namespace AgentTestCat.Views;

public partial class SettingsWindow : Window
{
    private readonly AppSettings _settings;
    private readonly PetSession _session;
    private readonly string? _projectRoot;
    private readonly string? _yamlPath;

    public SettingsWindow(AppSettings settings, PetSession session, string? projectRoot = null, string? yamlPath = null)
    {
        InitializeComponent();
        _settings = settings;
        _session = session;
        _projectRoot = projectRoot;
        _yamlPath = yamlPath;

        BaseUrlBox.Text = settings.BaseUrl;
        UsernameBox.Text = settings.Username;
        DeviceIdBox.Text = string.IsNullOrWhiteSpace(settings.DeviceId)
            ? "pc-" + Environment.MachineName
            : settings.DeviceId;
        RememberPasswordBox.IsChecked = settings.RememberPassword;
        DebugBox.IsChecked = settings.Debug;
        EnableSoundBox.IsChecked = settings.EnableSound;
        EnableReplyNotifyBox.IsChecked = settings.EnableReplyNotify;
        OpenWebUiOnStartupBox.IsChecked = settings.OpenWebUiOnStartup;
        if (settings.RememberPassword && !string.IsNullOrEmpty(settings.Password))
            PasswordBox.Password = settings.Password;

        UpdateStatusUi(session.IsConnected, session.IsConnected ? "当前已连接，可直接在桌宠输入框发消息。" : "未连接，请填写信息后点击「保存并连接」。");
    }

    private void UpdateStatusUi(bool connected, string message)
    {
        StatusDot.Fill = new SolidColorBrush(connected ? System.Windows.Media.Color.FromRgb(0x4C, 0xAF, 0x50) : System.Windows.Media.Color.FromRgb(0xB0, 0xA8, 0xA0));
        StatusText.Text = message;
    }

    private async void ConnectButton_Click(object sender, RoutedEventArgs e)
    {
        _settings.BaseUrl = BaseUrlBox.Text.Trim().TrimEnd('/');
        _settings.Username = UsernameBox.Text.Trim();
        _settings.DeviceId = DeviceIdBox.Text.Trim();
        _settings.RememberPassword = RememberPasswordBox.IsChecked == true;
        _settings.Debug = DebugBox.IsChecked == true;
        _settings.EnableSound = EnableSoundBox.IsChecked == true;
        _settings.EnableReplyNotify = EnableReplyNotifyBox.IsChecked == true;
        _settings.OpenWebUiOnStartup = OpenWebUiOnStartupBox.IsChecked == true;

        // 始终用密码框内容登录；仅「记住密码」时才写入 settings 文件
        var password = PasswordBox.Password;

        if (string.IsNullOrWhiteSpace(_settings.BaseUrl))
        {
            System.Windows.MessageBox.Show(this, "请填写服务地址", "提示", MessageBoxButton.OK, MessageBoxImage.Information);
            return;
        }
        if (string.IsNullOrWhiteSpace(password))
        {
            System.Windows.MessageBox.Show(this, "请填写密码", "提示", MessageBoxButton.OK, MessageBoxImage.Information);
            return;
        }

        _settings.Password = _settings.RememberPassword ? password : "";

        DebugLog.Close();
        DebugLog.Init(_settings.Debug);

        try
        {
            ConnectButton.IsEnabled = false;
            StatusText.Text = "正在连接…";
            StatusDot.Fill = new SolidColorBrush(System.Windows.Media.Color.FromRgb(0xFF, 0xA0, 0x26));

            await _session.ConnectAsync(_settings, password);
            _settings.Save();
            UpdateStatusUi(true, "已连接 · channel=desktop · " + _settings.BaseUrl);
        }
        catch (Exception ex)
        {
            UpdateStatusUi(false, "连接失败: " + ex.Message);
            System.Windows.MessageBox.Show(this, ex.Message, "连接失败", MessageBoxButton.OK, MessageBoxImage.Warning);
        }
        finally
        {
            ConnectButton.IsEnabled = true;
        }
    }

    private async void DisconnectButton_Click(object sender, RoutedEventArgs e)
    {
        await _session.DisconnectAsync();
        UpdateStatusUi(false, "已断开");
    }
}
