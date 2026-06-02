using System.Windows;
using AgentTestCat.Services;

namespace AgentTestCat.Views;

public partial class SetupWizardWindow : Window
{
    private readonly string _projectRoot;
    private readonly string _yamlPath;

    public string? DeepSeekKey { get; private set; }
    public string? DashScopeKey { get; private set; }
    public string? WebPassword { get; private set; }

    public SetupWizardWindow(string projectRoot, string yamlPath)
    {
        InitializeComponent();
        _projectRoot = projectRoot;
        _yamlPath = yamlPath;
        StatusText.Text = "配置目录：" + yamlPath;
    }

    private void ExitButton_Click(object sender, RoutedEventArgs e)
    {
        DialogResult = false;
        Close();
    }

    private void StartButton_Click(object sender, RoutedEventArgs e)
    {
        var deep = DeepSeekKeyBox.Password.Trim();
        if (string.IsNullOrEmpty(deep))
        {
            System.Windows.MessageBox.Show(this, "请填写 DeepSeek API Key（基本功能必需）。", "提示",
                MessageBoxButton.OK, MessageBoxImage.Information);
            return;
        }

        try
        {
            StartButton.IsEnabled = false;
            StatusText.Text = "正在写入配置…";

            var dash = DashScopeKeyBox.Password.Trim();
            var webPass = WebPasswordBox.Password;
            AppYamlConfigurator.ApplyApiKeys(
                _yamlPath,
                deep,
                string.IsNullOrEmpty(dash) ? null : dash,
                string.IsNullOrWhiteSpace(webPass) ? null : webPass.Trim());
            AppYamlConfigurator.NormalizeFileFormat(_yamlPath);

            DeepSeekKey = deep;
            DashScopeKey = string.IsNullOrEmpty(dash) ? null : dash;
            WebPassword = string.IsNullOrWhiteSpace(webPass) ? null : webPass.Trim();

            DialogResult = true;
            Close();
        }
        catch (Exception ex)
        {
            StatusText.Text = "保存失败：" + ex.Message;
            System.Windows.MessageBox.Show(this, ex.Message, "保存失败", MessageBoxButton.OK, MessageBoxImage.Warning);
        }
        finally
        {
            StartButton.IsEnabled = true;
        }
    }
}
