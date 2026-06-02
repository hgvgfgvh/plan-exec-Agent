using System.Globalization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Threading;
using AgentTestCat.Models;
using AgentTestCat.Services;
using Forms = System.Windows.Forms;
using OpenFileDialog = Microsoft.Win32.OpenFileDialog;

namespace AgentTestCat.Views;

public partial class PetWindow : Window
{
    private const int InitialInputChars = 8;
    private const int MaxCharsPerLine = 35;
    private const int MaxInputLines = 8;
    private const double CollapsedBaseHeight = 418;
    private const double CollapsedWindowWidth = 280;
    private const double MinExpandedWidth = 320;
    private const double MinExpandedHeight = 240;
    private const double MaxExpandedWidth = 960;
    private const double MaxExpandedHeight = 820;
    private const double InputLineHeight = 18;
    private const double InputPaddingH = 14;
    private const double ExpandedBottomReserve = 248;

    private enum ExpandedPanelAction { None, Move, ResizeRight, ResizeBottom, ResizeCorner }

    private ExpandedPanelAction _panelAction;
    private System.Windows.Point _panelDragStart;
    private double _panelStartWidth;
    private double _panelStartHeight;
    private Thickness _panelStartMargin;

    private readonly AppSettings _settings;
    private readonly PetSession _session;
    private readonly Action _openSettings;
    private readonly Action _exitApp;
    private readonly DispatcherTimer _animTimer;
    private readonly CatSoundService _sounds;
    private CatAnimator _catAnimator = null!;

    private bool _bubbleExpanded;
    private bool _inputUpdating;
    private bool _webViewReady;
    private double _inputMinWidth;
    private double _inputMaxWidth;

    private System.Windows.Point _catPressScreen;
    private bool _catDragStarted;

    public PetWindow(AppSettings settings, PetSession session, CatSoundService sounds, Action openSettings, Action exitApp)
    {
        InitializeComponent();
        _settings = settings;
        _session = session;
        _sounds = sounds;
        _openSettings = openSettings;
        _exitApp = exitApp;

        Left = settings.WindowLeft;
        Top = settings.WindowTop;

        CatIconHelper.ApplyWindowIcon(this);

        _inputMinWidth = MeasureTextWidth(new string('中', InitialInputChars)) + InputPaddingH;
        _inputMaxWidth = MeasureTextWidth(new string('中', MaxCharsPerLine)) + InputPaddingH;
        MessageBox.Width = _inputMinWidth;
        MessageBox.MinWidth = _inputMinWidth;

        _session.HistoryUpdated += () => Dispatcher.Invoke(RefreshHistoryUi);
        _session.StatusChanged += msg =>
        {
            if (string.IsNullOrWhiteSpace(msg)) return;
            Dispatcher.Invoke(() =>
            {
                ConnectionStatusText.Text = msg.Trim();
                Title = "AgentTest Cat · " + msg.Trim();
            });
        };
        _session.ConnectionChanged += (connected, detail) =>
            Dispatcher.Invoke(() => UpdateConnectionStatus(connected, detail));
        _session.SseUnstable += msg =>
            Dispatcher.Invoke(() => UpdateConnectionStatus(_session.IsConnected, msg, unstable: true));
        _session.MoodChanged += mood => Dispatcher.Invoke(() => UpdateMood(mood));

        _animTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(380) };
        _animTimer.Start();

        _catAnimator = new CatAnimator(
            CatImage, CatScale, CatTranslate, CatFxCanvas, CatQuipBubble, CatQuipText, _animTimer);
        CatImage.Source = CatSpriteCatalog.Idle;

        RefreshHistoryUi();
        AdjustInputLayout();

        // 确保输入框等子控件不吞掉拖放（Preview 已在 Window/Border/TextBox 注册）
        MessageBox.AllowDrop = true;

        UpdateConnectionStatus(_session.IsConnected, _session.IsConnected ? "已连接" : "未连接");
    }

    public void UpdateConnectionStatus(bool connected, string detail, bool unstable = false)
    {
        ConnectionStatusText.Text = detail;
        Title = "AgentTest Cat · " + detail;
        var color = connected
            ? (unstable
                ? System.Windows.Media.Color.FromRgb(0xFF, 0xA0, 0x26)
                : System.Windows.Media.Color.FromRgb(0x4C, 0xAF, 0x50))
            : System.Windows.Media.Color.FromRgb(0x80, 0x78, 0x70);
        ConnectionStatusDot.Fill = new SolidColorBrush(color);
        ConnectionStatusDot.ToolTip = connected
            ? (unstable ? "已连接（SSE 重连中）" : "已连接 AgentTest")
            : "未连接 · 点击齿轮设置";
    }

    public void PrepareForShutdown() => _animTimer.Stop();

    private void RefreshHistoryUi()
    {
        HistoryList.Items.Clear();
        foreach (var m in _session.History.Messages.TakeLast(8))
        {
            var panel = new StackPanel { Margin = new Thickness(0, 2, 0, 2) };
            var label = new TextBlock
            {
                Text = m.IsUser ? "你" : m.Source + (m.IsStreaming ? " · …" : ""),
                FontSize = 10,
                Foreground = new SolidColorBrush(System.Windows.Media.Color.FromRgb(0xA8, 0xA4, 0xA0))
            };
            var preview = MarkdownHelper.PreviewPlain(m.IsUser ? "你" : m.Source, m.Text, 80);
            var body = new TextBlock
            {
                Text = preview,
                TextWrapping = TextWrapping.Wrap,
                FontSize = 11,
                Foreground = new SolidColorBrush(System.Windows.Media.Color.FromRgb(0xF0, 0xEE, 0xEB)),
                MaxHeight = m.IsUser ? 32 : 48,
                TextTrimming = TextTrimming.CharacterEllipsis
            };
            panel.Children.Add(label);
            panel.Children.Add(body);
            HistoryList.Items.Add(panel);
        }

        ExpandButton.Visibility = _session.History.NeedsExpand() ? Visibility.Visible : Visibility.Collapsed;

        AttachChips.Items.Clear();
        if (_session.Attachments.Items.Count > 0)
        {
            AttachChips.Visibility = Visibility.Visible;
            foreach (var a in _session.Attachments.Items)
            {
                var attachmentId = a.Id;
                var chip = new Border
                {
                    Background = new SolidColorBrush(System.Windows.Media.Color.FromArgb(0xCC, 0x44, 0x40, 0x50)),
                    CornerRadius = new CornerRadius(10),
                    Padding = new Thickness(8, 3, 4, 3),
                    Margin = new Thickness(0, 0, 4, 4),
                    Cursor = System.Windows.Input.Cursors.Arrow
                };

                var grid = new Grid();
                grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });

                var label = new TextBlock
                {
                    Text = (a.Kind == "image" ? "🖼 " : a.Kind == "folder" ? "📁 " : "📄 ") + a.DisplayName,
                    FontSize = 10,
                    Foreground = System.Windows.Media.Brushes.WhiteSmoke,
                    MaxWidth = 120,
                    TextTrimming = TextTrimming.CharacterEllipsis,
                    VerticalAlignment = VerticalAlignment.Center
                };
                Grid.SetColumn(label, 0);
                grid.Children.Add(label);

                var removeBtn = new System.Windows.Controls.Button
                {
                    Content = "✕",
                    Width = 16,
                    Height = 16,
                    Padding = new Thickness(0),
                    Margin = new Thickness(4, 0, 0, 0),
                    FontSize = 9,
                    Foreground = System.Windows.Media.Brushes.White,
                    Background = new SolidColorBrush(System.Windows.Media.Color.FromRgb(0xB0, 0x40, 0x48)),
                    BorderThickness = new Thickness(0),
                    Cursor = System.Windows.Input.Cursors.Hand,
                    Visibility = Visibility.Collapsed,
                    VerticalAlignment = VerticalAlignment.Center,
                    ToolTip = "移除此附件"
                };
                removeBtn.Click += async (_, _) =>
                {
                    try { await _session.RemoveAttachmentAsync(attachmentId); }
                    catch (Exception ex)
                    {
                        _session.History.AddSystem("移除失败: " + ex.Message);
                        RefreshHistoryUi();
                    }
                };
                Grid.SetColumn(removeBtn, 1);
                grid.Children.Add(removeBtn);

                chip.Child = grid;
                chip.MouseEnter += (_, _) => removeBtn.Visibility = Visibility.Visible;
                chip.MouseLeave += (_, _) => removeBtn.Visibility = Visibility.Collapsed;

                AttachChips.Items.Add(chip);
            }
        }
        else
        {
            AttachChips.Visibility = Visibility.Collapsed;
        }

        Dispatcher.BeginInvoke(() =>
        {
            HistoryScroll.ScrollToEnd();
        }, DispatcherPriority.Loaded);

        if (_bubbleExpanded)
            _ = RenderExpandedHistoryAsync();
    }

    private void UpdateMood(PetMood mood) => _catAnimator.SetMood(mood);

    private async void ExpandButton_Click(object sender, RoutedEventArgs e) =>
        await SetExpandedAsync(true);

    private async void CollapseButton_Click(object sender, RoutedEventArgs e) =>
        await SetExpandedAsync(false);

    private async void ClearHistoryButton_Click(object sender, RoutedEventArgs e)
    {
        if (_session.History.Messages.Count == 0) return;

        var result = System.Windows.MessageBox.Show(
            this,
            "确定清空全部历史对话吗？此操作不可恢复。",
            "清空历史",
            MessageBoxButton.YesNo,
            MessageBoxImage.Question,
            MessageBoxResult.No);
        if (result != MessageBoxResult.Yes) return;

        _session.ClearHistory();
        RefreshHistoryUi();
        if (_bubbleExpanded)
            await RenderExpandedHistoryAsync();
    }

    private async Task SetExpandedAsync(bool expanded)
    {
        _bubbleExpanded = expanded;
        if (expanded)
        {
            ApplyExpandedPanelLayout();
            BubbleExpanded.Visibility = Visibility.Visible;
            SyncExpandedWindowSize();
            await RenderExpandedHistoryAsync();
        }
        else
        {
            SaveExpandedPanelLayout();
            BubbleExpanded.Visibility = Visibility.Collapsed;
            Width = CollapsedWindowWidth;
            AdjustWindowSize();
        }
    }

    private void ApplyExpandedPanelLayout()
    {
        var w = Clamp(_settings.ExpandedPanelWidth, MinExpandedWidth, MaxExpandedWidth);
        var h = Clamp(_settings.ExpandedPanelHeight, MinExpandedHeight, MaxExpandedHeight);
        BubbleExpanded.Width = w;
        BubbleExpanded.Height = h;
        BubbleExpanded.Margin = new Thickness(
            Math.Max(0, _settings.ExpandedPanelOffsetX),
            Math.Max(0, _settings.ExpandedPanelOffsetY), 0, 0);
    }

    private void SaveExpandedPanelLayout()
    {
        _settings.ExpandedPanelWidth = BubbleExpanded.Width;
        _settings.ExpandedPanelHeight = BubbleExpanded.Height;
        _settings.ExpandedPanelOffsetX = BubbleExpanded.Margin.Left;
        _settings.ExpandedPanelOffsetY = BubbleExpanded.Margin.Top;
        _settings.Save();
    }

    private void SyncExpandedWindowSize()
    {
        if (!_bubbleExpanded) return;
        var lines = Math.Clamp(CountInputLines(MessageBox.Text), 1, MaxInputLines);
        var inputExtra = (lines - 1) * InputLineHeight;
        var neededW = BubbleExpanded.Margin.Left + BubbleExpanded.Width + 16;
        var neededH = BubbleExpanded.Margin.Top + BubbleExpanded.Height + ExpandedBottomReserve + inputExtra;
        Width = Math.Max(CollapsedWindowWidth, neededW);
        Height = Math.Max(CollapsedBaseHeight + inputExtra, neededH);
    }

    private static double Clamp(double v, double min, double max) => Math.Max(min, Math.Min(max, v));

    private void ExpandedPanel_HeaderMouseDown(object sender, MouseButtonEventArgs e)
    {
        if (e.ButtonState != MouseButtonState.Pressed) return;
        BeginPanelAction(ExpandedPanelAction.Move, e);
        e.Handled = true;
    }

    private void ExpandedPanel_ResizeRight(object sender, MouseButtonEventArgs e)
    {
        if (e.ButtonState != MouseButtonState.Pressed) return;
        BeginPanelAction(ExpandedPanelAction.ResizeRight, e);
        e.Handled = true;
    }

    private void ExpandedPanel_ResizeBottom(object sender, MouseButtonEventArgs e)
    {
        if (e.ButtonState != MouseButtonState.Pressed) return;
        BeginPanelAction(ExpandedPanelAction.ResizeBottom, e);
        e.Handled = true;
    }

    private void ExpandedPanel_ResizeCorner(object sender, MouseButtonEventArgs e)
    {
        if (e.ButtonState != MouseButtonState.Pressed) return;
        BeginPanelAction(ExpandedPanelAction.ResizeCorner, e);
        e.Handled = true;
    }

    private void BeginPanelAction(ExpandedPanelAction action, MouseButtonEventArgs e)
    {
        _panelAction = action;
        _panelDragStart = e.GetPosition(WindowRoot);
        _panelStartWidth = BubbleExpanded.Width;
        _panelStartHeight = BubbleExpanded.Height;
        _panelStartMargin = BubbleExpanded.Margin;
        BubbleExpanded.CaptureMouse();
        CaptureMouse();
    }

    private void WindowRoot_MouseMove(object sender, System.Windows.Input.MouseEventArgs e)
    {
        if (_panelAction == ExpandedPanelAction.None || e.LeftButton != MouseButtonState.Pressed)
            return;

        var pos = e.GetPosition(WindowRoot);
        var dx = pos.X - _panelDragStart.X;
        var dy = pos.Y - _panelDragStart.Y;

        switch (_panelAction)
        {
            case ExpandedPanelAction.Move:
                BubbleExpanded.Margin = new Thickness(
                    Math.Max(0, _panelStartMargin.Left + dx),
                    Math.Max(0, _panelStartMargin.Top + dy), 0, 0);
                break;
            case ExpandedPanelAction.ResizeRight:
                BubbleExpanded.Width = Clamp(_panelStartWidth + dx, MinExpandedWidth, MaxExpandedWidth);
                break;
            case ExpandedPanelAction.ResizeBottom:
                BubbleExpanded.Height = Clamp(_panelStartHeight + dy, MinExpandedHeight, MaxExpandedHeight);
                break;
            case ExpandedPanelAction.ResizeCorner:
                BubbleExpanded.Width = Clamp(_panelStartWidth + dx, MinExpandedWidth, MaxExpandedWidth);
                BubbleExpanded.Height = Clamp(_panelStartHeight + dy, MinExpandedHeight, MaxExpandedHeight);
                break;
        }

        SyncExpandedWindowSize();
    }

    private void WindowRoot_MouseLeftButtonUp(object sender, MouseButtonEventArgs e)
    {
        if (_panelAction == ExpandedPanelAction.None) return;
        _panelAction = ExpandedPanelAction.None;
        BubbleExpanded.ReleaseMouseCapture();
        ReleaseMouseCapture();
        SaveExpandedPanelLayout();
    }

    private async Task EnsureWebViewAsync()
    {
        if (_webViewReady) return;
        await MarkdownWebView.EnsureCoreWebView2Async();
        _webViewReady = true;
        MarkdownWebView.NavigationCompleted += async (_, _) =>
        {
            try
            {
                await MarkdownWebView.CoreWebView2.ExecuteScriptAsync(
                    "document.documentElement.style.overflowX='hidden';document.body.style.overflowX='hidden';");
            }
            catch { /* ignore */ }
        };
    }

    private async Task RenderExpandedHistoryAsync()
    {
        await EnsureWebViewAsync();
        var html = MarkdownHtmlRenderer.ToConversationHtml(_session.History.Messages);
        MarkdownWebView.NavigateToString(html);
    }

    private void AttachButton_Click(object sender, RoutedEventArgs e)
    {
        var menu = new ContextMenu();
        var files = new MenuItem { Header = "上传文件" };
        files.Click += (_, _) => PickFiles();
        var folder = new MenuItem { Header = "上传文件夹" };
        folder.Click += (_, _) => PickFolder();
        var shot = new MenuItem { Header = "框选截图" };
        shot.Click += (_, _) => _ = CaptureScreenshotAsync();
        menu.Items.Add(files);
        menu.Items.Add(folder);
        menu.Items.Add(shot);
        menu.PlacementTarget = AttachButton;
        menu.IsOpen = true;
    }

    private void PickFiles()
    {
        var dlg = new OpenFileDialog { Multiselect = true, Title = "选择文件" };
        if (dlg.ShowDialog() != true) return;
        _ = UploadPathsAsync(dlg.FileNames);
    }

    private void PickFolder()
    {
        using var dlg = new Forms.FolderBrowserDialog
        {
            Description = "选择文件夹",
            UseDescriptionForTitle = true
        };
        if (dlg.ShowDialog() != Forms.DialogResult.OK) return;
        _ = UploadPathsAsync(new[] { dlg.SelectedPath });
    }

    private async void ScreenshotButton_Click(object sender, RoutedEventArgs e) =>
        await CaptureScreenshotAsync();

    private async Task CaptureScreenshotAsync()
    {
        try
        {
            Hide();
            await Task.Delay(120);
            var overlay = new RegionCaptureWindow();
            var ok = overlay.ShowDialog();
            Show();
            Activate();
            if (ok != true || string.IsNullOrEmpty(overlay.SavedPath)) return;
            await UploadPathsAsync(new[] { overlay.SavedPath });
        }
        catch (Exception ex)
        {
            Show();
            Activate();
            _session.History.AddSystem("截图失败: " + ex.Message);
            RefreshHistoryUi();
        }
    }

    private async Task UploadPathsAsync(IEnumerable<string> paths)
    {
        try
        {
            await _session.UploadPathsAsync(paths);
        }
        catch (Exception ex)
        {
            _session.History.AddSystem("上传失败: " + ex.Message);
            RefreshHistoryUi();
        }
    }

    private void FileDrop_PreviewDragLeave(object sender, System.Windows.DragEventArgs e) =>
        SetDropHighlight(false);

    private void FileDrop_PreviewDragOver(object sender, System.Windows.DragEventArgs e)
    {
        if (_bubbleExpanded)
        {
            e.Effects = System.Windows.DragDropEffects.None;
            e.Handled = true;
            return;
        }

        if (TryExtractFileDropPaths(e, out _))
        {
            e.Effects = System.Windows.DragDropEffects.Copy;
            e.Handled = true;
            SetDropHighlight(true);
        }
        else
        {
            e.Effects = System.Windows.DragDropEffects.None;
        }
    }

    private async void FileDrop_PreviewDrop(object sender, System.Windows.DragEventArgs e)
    {
        SetDropHighlight(false);
        if (_bubbleExpanded) return;
        if (!TryExtractFileDropPaths(e, out var paths)) return;
        e.Handled = true;
        await UploadPathsAsync(paths);
    }

    private void SetDropHighlight(bool active)
    {
        if (active)
        {
            InputArea.Background = new SolidColorBrush(System.Windows.Media.Color.FromArgb(0x44, 0xC4, 0xA8, 0x82));
        }
        else
        {
            InputArea.Background = null;
        }
    }

    private static bool TryExtractFileDropPaths(System.Windows.DragEventArgs e, out string[] paths)
    {
        paths = Array.Empty<string>();
        if (!e.Data.GetDataPresent(System.Windows.DataFormats.FileDrop))
            return false;

        if (e.Data.GetData(System.Windows.DataFormats.FileDrop) is not string[] raw || raw.Length == 0)
            return false;

        paths = raw.Where(p => !string.IsNullOrWhiteSpace(p)).ToArray();
        return paths.Length > 0;
    }

    private void MessageBox_TextChanged(object sender, TextChangedEventArgs e)
    {
        if (_inputUpdating) return;

        var wrapped = MarkdownHelper.EnforceMaxCharsPerLine(MessageBox.Text, MaxCharsPerLine);
        if (!string.Equals(wrapped, MessageBox.Text, StringComparison.Ordinal))
        {
            _inputUpdating = true;
            var caret = MessageBox.CaretIndex;
            var delta = wrapped.Length - MessageBox.Text.Length;
            MessageBox.Text = wrapped;
            MessageBox.CaretIndex = Math.Clamp(caret + Math.Max(0, delta), 0, wrapped.Length);
            _inputUpdating = false;
        }

        if (CountInputLines(MessageBox.Text) > MaxInputLines)
        {
            _inputUpdating = true;
            var lines = MessageBox.Text.Replace("\r\n", "\n").Split('\n');
            MessageBox.Text = string.Join("\n", lines.Take(MaxInputLines));
            MessageBox.CaretIndex = MessageBox.Text.Length;
            _inputUpdating = false;
        }

        AdjustInputLayout();
    }

    private void AdjustInputLayout()
    {
        var text = MessageBox.Text ?? "";
        var lines = string.IsNullOrEmpty(text) ? new[] { "" } : text.Replace("\r\n", "\n").Split('\n');
        var longestLine = lines.OrderByDescending(static l => l.Length).FirstOrDefault() ?? "";
        var sample = longestLine.Length > 0 ? longestLine : new string('中', InitialInputChars);
        var inputWidth = Math.Clamp(MeasureTextWidth(sample) + InputPaddingH, _inputMinWidth, _inputMaxWidth);

        MessageBox.Width = inputWidth;
        var lineCount = Math.Clamp(lines.Length, 1, MaxInputLines);
        MessageBox.Height = 28 + (lineCount - 1) * InputLineHeight;

        if (!_bubbleExpanded)
            AdjustWindowSize();
    }

    private void AdjustWindowSize()
    {
        var lines = Math.Clamp(CountInputLines(MessageBox.Text), 1, MaxInputLines);
        Height = CollapsedBaseHeight + (lines - 1) * InputLineHeight;
    }

    private double MeasureTextWidth(string text)
    {
        if (string.IsNullOrEmpty(text)) return _inputMinWidth - InputPaddingH;
        var dpi = VisualTreeHelper.GetDpi(this).PixelsPerDip;
        var ft = new FormattedText(
            text,
            CultureInfo.CurrentUICulture,
            System.Windows.FlowDirection.LeftToRight,
            new Typeface(MessageBox.FontFamily, FontStyles.Normal, FontWeights.Normal, FontStretches.Normal),
            MessageBox.FontSize,
            System.Windows.Media.Brushes.White,
            dpi);
        return ft.WidthIncludingTrailingWhitespace;
    }

    private static int CountInputLines(string text)
    {
        if (string.IsNullOrEmpty(text)) return 1;
        return text.Replace("\r\n", "\n").Split('\n').Length;
    }

    private void MessageBox_PreviewKeyDown(object sender, System.Windows.Input.KeyEventArgs e)
    {
        if (e.Key != System.Windows.Input.Key.Enter) return;
        if (Keyboard.Modifiers.HasFlag(ModifierKeys.Shift)) return;
        e.Handled = true;
        _ = SendAsync();
    }

    private void CatDragArea_MouseLeftButtonDown(object sender, MouseButtonEventArgs e)
    {
        if (e.ButtonState != MouseButtonState.Pressed) return;
        _catPressScreen = PointToScreen(e.GetPosition(CatDragArea));
        _catDragStarted = false;
        CatDragArea.CaptureMouse();
        _catAnimator.NotifyActivity();
    }

    private void CatDragArea_MouseMove(object sender, System.Windows.Input.MouseEventArgs e)
    {
        if (e.LeftButton != MouseButtonState.Pressed || _catDragStarted) return;
        var now = PointToScreen(e.GetPosition(CatDragArea));
        var dx = now.X - _catPressScreen.X;
        var dy = now.Y - _catPressScreen.Y;
        if (Math.Abs(dx) + Math.Abs(dy) < 6) return;
        _catDragStarted = true;
        CatDragArea.ReleaseMouseCapture();
        DragMove();
    }

    private void CatDragArea_MouseLeftButtonUp(object sender, MouseButtonEventArgs e)
    {
        if (CatDragArea.IsMouseCaptured)
            CatDragArea.ReleaseMouseCapture();
        if (!_catDragStarted)
        {
            if (e.ClickCount >= 2)
                _ = SetExpandedAsync(true);
            else
            {
                _catAnimator.OnPoke(e.ClickCount);
                _sounds.PlayMeowShort();
            }
        }
        _catDragStarted = false;
    }

    private void GearButton_Click(object sender, RoutedEventArgs e) => _openSettings();

    private void ExitButton_Click(object sender, RoutedEventArgs e) =>
        Dispatcher.BeginInvoke(_exitApp, DispatcherPriority.Send);

    private async void SendButton_Click(object sender, RoutedEventArgs e) => await SendAsync();

    private async Task SendAsync()
    {
        var text = MessageBox.Text.Trim();
        if (string.IsNullOrEmpty(text) && !_session.HasPendingAttachments) return;
        MessageBox.Clear();
        AdjustInputLayout();
        _catAnimator.NotifyActivity();
        try
        {
            await _session.SendAsync(text);
        }
        catch (Exception ex)
        {
            _session.History.AddSystem(ex.Message);
            RefreshHistoryUi();
        }
    }

    protected override void OnClosing(System.ComponentModel.CancelEventArgs e)
    {
        _settings.WindowLeft = Left;
        _settings.WindowTop = Top;
        _settings.Save();
        base.OnClosing(e);
    }
}
