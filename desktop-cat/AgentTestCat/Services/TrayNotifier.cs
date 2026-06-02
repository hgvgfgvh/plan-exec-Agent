using System.Windows;
using Forms = System.Windows.Forms;

namespace AgentTestCat.Services;

public sealed class TrayNotifier
{
    private readonly AppSettings _settings;
    private readonly Forms.NotifyIcon _tray;
    private readonly PetSession _session;
    private readonly CatSoundService _sounds;
    private readonly Func<Window?> _getWindow;
    private bool _wasConnected;

    public TrayNotifier(
        AppSettings settings,
        Forms.NotifyIcon tray,
        PetSession session,
        CatSoundService sounds,
        Func<Window?> getWindow)
    {
        _settings = settings;
        _tray = tray;
        _session = session;
        _sounds = sounds;
        _getWindow = getWindow;
        _wasConnected = session.IsConnected;

        _session.ConnectionChanged += OnConnectionChanged;
        _session.StatusChanged += UpdateTrayText;
        _session.ReplyCompleted += OnReplyCompleted;
        _session.SseUnstable += OnSseUnstable;

        UpdateTrayText(_session.IsConnected ? "已连接" : "未连接");
    }

    private void OnConnectionChanged(bool connected, string detail)
    {
        System.Windows.Application.Current?.Dispatcher.Invoke(() =>
        {
            UpdateTrayText(detail);
            // 仅「从已连接变为断开」时弹窗，避免重连/启动误报
            if (!connected && _wasConnected && _settings.EnableReplyNotify)
                ShowBalloon("连接已断开", detail, Forms.ToolTipIcon.Warning);
            _wasConnected = connected;
        });
    }

    private void OnSseUnstable(string message)
    {
        System.Windows.Application.Current?.Dispatcher.Invoke(() =>
        {
            UpdateTrayText(message);
            if (_session.IsConnected && _settings.EnableReplyNotify)
                ShowBalloon("连接不稳定", message, Forms.ToolTipIcon.Warning);
        });
    }

    private void OnReplyCompleted(string preview)
    {
        System.Windows.Application.Current?.Dispatcher.Invoke(() =>
        {
            var win = _getWindow();
            var hiddenOrUnfocused = win == null || !win.IsVisible || !win.IsActive;
            if (!hiddenOrUnfocused) return;
            if (!_settings.EnableReplyNotify) return;

            ShowBalloon("Agent 回复完成", preview, Forms.ToolTipIcon.Info);
            _sounds.PlayMeow();
            FlashWindow(win);
        });
    }

    private void UpdateTrayText(string detail)
    {
        if (string.IsNullOrWhiteSpace(detail)) return;
        var prefix = _session.IsConnected ? "● 已连接" : "○ 未连接";
        _tray.Text = Truncate($"{prefix} · {detail.Trim()}", 63);
    }

    private void ShowBalloon(string title, string text, Forms.ToolTipIcon icon)
    {
        try
        {
            _tray.ShowBalloonTip(4000, title, Truncate(text, 240), icon);
        }
        catch
        {
            /* Win11 可能限制 balloon */
        }
    }

    private static void FlashWindow(Window? win)
    {
        if (win == null || !win.IsVisible) return;
        try
        {
            var helper = new System.Windows.Interop.WindowInteropHelper(win);
            if (helper.Handle == IntPtr.Zero) return;
            FlashWindowApi(helper.Handle, true);
        }
        catch { /* ignore */ }
    }

    [System.Runtime.InteropServices.DllImport("user32.dll")]
    private static extern bool FlashWindowApi(IntPtr hwnd, bool invert);

    private static string Truncate(string s, int max) =>
        s.Length <= max ? s : s[..(max - 1)] + "…";
}
