using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Media.Animation;
using System.Windows.Shapes;
using MediaColor = System.Windows.Media.Color;
using MediaFontFamily = System.Windows.Media.FontFamily;

namespace AgentTestCat.Services;

/// <summary>代码绘制的小特效：爱心、问号、Zzz、汗滴等。</summary>
public static class CatFxOverlay
{
    public static void SpawnHeart(Canvas canvas, double x, double y)
    {
        var heart = new Path
        {
            Width = 12,
            Height = 12,
            Stretch = Stretch.Uniform,
            Fill = new SolidColorBrush(MediaColor.FromRgb(0xE8, 0x68, 0x88)),
            Data = Geometry.Parse("M6,10 C6,10 1,6 1,4 A2.2,2.2 0 0 1 6,2 A2.2,2.2 0 0 1 11,4 C11,6 6,10 6,10 Z")
        };
        Canvas.SetLeft(heart, x - 6);
        Canvas.SetTop(heart, y);
        canvas.Children.Add(heart);
        AnimateFloatFade(canvas, heart, -36, 900);
    }

    public static void SpawnQuestion(Canvas canvas, double x, double y)
    {
        var tb = new TextBlock
        {
            Text = "?",
            FontFamily = new MediaFontFamily("Segoe UI"),
            FontWeight = FontWeights.Bold,
            FontSize = 16,
            Foreground = new SolidColorBrush(MediaColor.FromRgb(0xF0, 0xEE, 0xEB)),
            Opacity = 0.95
        };
        Canvas.SetLeft(tb, x - 4);
        Canvas.SetTop(tb, y);
        canvas.Children.Add(tb);
        AnimateFloatFade(canvas, tb, -28, 1100, driftX: 6);
    }

    public static void SpawnZzz(Canvas canvas, double x, double y)
    {
        var tb = new TextBlock
        {
            Text = "Z",
            FontFamily = new MediaFontFamily("Segoe UI"),
            FontWeight = FontWeights.SemiBold,
            FontSize = 12,
            Foreground = new SolidColorBrush(MediaColor.FromRgb(0xA8, 0xA4, 0xA0)),
            Opacity = 0.85
        };
        Canvas.SetLeft(tb, x);
        Canvas.SetTop(tb, y);
        canvas.Children.Add(tb);
        AnimateFloatFade(canvas, tb, -22, 1400, driftX: 8);
    }

    public static void SpawnSweat(Canvas canvas, double x, double y)
    {
        var drop = new Ellipse
        {
            Width = 6,
            Height = 8,
            Fill = new SolidColorBrush(MediaColor.FromArgb(0xCC, 0x80, 0xC0, 0xE8)),
            Stroke = new SolidColorBrush(MediaColor.FromArgb(0x88, 0x40, 0x80, 0xA0)),
            StrokeThickness = 1
        };
        Canvas.SetLeft(drop, x);
        Canvas.SetTop(drop, y);
        canvas.Children.Add(drop);
        AnimateFloatFade(canvas, drop, 18, 800);
    }

    public static void BurstHearts(Canvas canvas, double centerX, double centerY, int count = 3)
    {
        for (var i = 0; i < count; i++)
        {
            var ox = (i - 1) * 14 + Random.Shared.Next(-4, 5);
            SpawnHeart(canvas, centerX + ox, centerY - 8);
        }
    }

    private static void AnimateFloatFade(Canvas canvas, FrameworkElement el, double dy, int ms, double driftX = 0)
    {
        var startTop = Canvas.GetTop(el);
        if (double.IsNaN(startTop)) startTop = 0;
        var startLeft = Canvas.GetLeft(el);
        if (double.IsNaN(startLeft)) startLeft = 0;

        var fade = new DoubleAnimation(el.Opacity, 0, TimeSpan.FromMilliseconds(ms))
        {
            EasingFunction = new QuadraticEase { EasingMode = EasingMode.EaseIn }
        };
        var moveY = new DoubleAnimation(startTop, startTop + dy, TimeSpan.FromMilliseconds(ms))
        {
            EasingFunction = new QuadraticEase { EasingMode = EasingMode.EaseOut }
        };

        el.BeginAnimation(UIElement.OpacityProperty, fade);
        el.BeginAnimation(Canvas.TopProperty, moveY);
        if (driftX != 0)
        {
            var moveX = new DoubleAnimation(startLeft, startLeft + driftX, TimeSpan.FromMilliseconds(ms));
            el.BeginAnimation(Canvas.LeftProperty, moveX);
        }

        moveY.Completed += (_, _) => canvas.Children.Remove(el);
    }
}
