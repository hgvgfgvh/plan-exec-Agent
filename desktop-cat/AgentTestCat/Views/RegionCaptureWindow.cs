using System.Drawing;
using System.Drawing.Imaging;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Shapes;
using Brushes = System.Windows.Media.Brushes;

namespace AgentTestCat.Views;

/// <summary>全屏框选截图 overlay。</summary>
public sealed class RegionCaptureWindow : Window
{
    private System.Windows.Point _start;
    private bool _dragging;
    private readonly Canvas _canvas;
    private readonly System.Windows.Shapes.Rectangle _selection;
    private readonly TextBlock _hint;

    public string? SavedPath { get; private set; }

    public RegionCaptureWindow()
    {
        WindowStyle = WindowStyle.None;
        AllowsTransparency = true;
        Background = Brushes.Transparent;
        Topmost = true;
        ShowInTaskbar = false;
        Cursor = System.Windows.Input.Cursors.Cross;

        Left = SystemParameters.VirtualScreenLeft;
        Top = SystemParameters.VirtualScreenTop;
        Width = SystemParameters.VirtualScreenWidth;
        Height = SystemParameters.VirtualScreenHeight;

        var root = new Grid();
        root.Background = new SolidColorBrush(System.Windows.Media.Color.FromArgb(0x55, 0, 0, 0));

        _canvas = new Canvas { Background = Brushes.Transparent };
        _selection = new System.Windows.Shapes.Rectangle
        {
            Stroke = Brushes.White,
            StrokeThickness = 2,
            Fill = new SolidColorBrush(System.Windows.Media.Color.FromArgb(0x33, 0xFF, 0xFF, 0xFF)),
            Visibility = Visibility.Collapsed
        };
        _canvas.Children.Add(_selection);

        _hint = new TextBlock
        {
            Text = "拖动鼠标框选区域 · Esc 取消",
            Foreground = Brushes.White,
            FontSize = 14,
            HorizontalAlignment = System.Windows.HorizontalAlignment.Center,
            VerticalAlignment = VerticalAlignment.Top,
            Margin = new Thickness(0, 24, 0, 0)
        };

        root.Children.Add(_canvas);
        root.Children.Add(_hint);
        Content = root;

        MouseLeftButtonDown += OnMouseDown;
        MouseMove += OnMouseMove;
        MouseLeftButtonUp += OnMouseUp;
        KeyDown += OnKeyDown;
    }

    private void OnMouseDown(object sender, MouseButtonEventArgs e)
    {
        _start = e.GetPosition(_canvas);
        _dragging = true;
        Canvas.SetLeft(_selection, _start.X);
        Canvas.SetTop(_selection, _start.Y);
        _selection.Width = 0;
        _selection.Height = 0;
        _selection.Visibility = Visibility.Visible;
        CaptureMouse();
    }

    private void OnMouseMove(object sender, System.Windows.Input.MouseEventArgs e)
    {
        if (!_dragging) return;
        var cur = e.GetPosition(_canvas);
        var x = Math.Min(_start.X, cur.X);
        var y = Math.Min(_start.Y, cur.Y);
        var w = Math.Abs(cur.X - _start.X);
        var h = Math.Abs(cur.Y - _start.Y);
        Canvas.SetLeft(_selection, x);
        Canvas.SetTop(_selection, y);
        _selection.Width = w;
        _selection.Height = h;
    }

    private void OnMouseUp(object sender, MouseButtonEventArgs e)
    {
        if (!_dragging) return;
        _dragging = false;
        ReleaseMouseCapture();

        var cur = e.GetPosition(_canvas);
        var p1 = PointToScreen(_start);
        var p2 = PointToScreen(cur);
        var x = (int)Math.Round(Math.Min(p1.X, p2.X));
        var y = (int)Math.Round(Math.Min(p1.Y, p2.Y));
        var w = (int)Math.Round(Math.Abs(p2.X - p1.X));
        var h = (int)Math.Round(Math.Abs(p2.Y - p1.Y));

        if (w < 8 || h < 8)
        {
            DialogResult = false;
            Close();
            return;
        }

        try
        {
            SavedPath = CaptureRectToTempPng(x, y, w, h);
            DialogResult = true;
        }
        catch
        {
            DialogResult = false;
        }
        Close();
    }

    private void OnKeyDown(object sender, System.Windows.Input.KeyEventArgs e)
    {
        if (e.Key == Key.Escape)
        {
            DialogResult = false;
            Close();
        }
    }

    private static string CaptureRectToTempPng(int x, int y, int w, int h)
    {
        using var bmp = new Bitmap(w, h);
        using (var g = Graphics.FromImage(bmp))
            g.CopyFromScreen(x, y, 0, 0, new System.Drawing.Size(w, h), CopyPixelOperation.SourceCopy);

        var path = System.IO.Path.Combine(System.IO.Path.GetTempPath(), $"agenttest-cat-{Guid.NewGuid():N}.png");
        bmp.Save(path, ImageFormat.Png);
        return path;
    }
}
