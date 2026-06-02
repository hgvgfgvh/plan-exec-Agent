using System.Drawing;
using System.IO;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;

namespace AgentTestCat.Services;

public static class CatIconHelper
{
    private static Icon? _trayIcon;

    public static System.Windows.Media.ImageSource? LoadCatImageSource()
    {
        try
        {
            return CatSpriteCatalog.Idle;
        }
        catch
        {
            try
            {
                return BitmapFrame.Create(new Uri("pack://application:,,,/Assets/cat_pixel.png"));
            }
            catch
            {
                return null;
            }
        }
    }

    public static void ApplyWindowIcon(Window window)
    {
        var src = LoadCatImageSource();
        if (src != null)
            window.Icon = src;
    }

    public static Icon GetTrayIcon()
    {
        if (_trayIcon != null) return _trayIcon;
        var uri = new Uri("pack://application:,,,/Assets/cat_pixel.png");
        var decoder = BitmapDecoder.Create(uri, BitmapCreateOptions.None, BitmapCacheOption.OnLoad);
        var frame = decoder.Frames[0];
        var bmp = new Bitmap(frame.PixelWidth, frame.PixelHeight, System.Drawing.Imaging.PixelFormat.Format32bppArgb);
        var data = bmp.LockBits(
            new Rectangle(0, 0, bmp.Width, bmp.Height),
            System.Drawing.Imaging.ImageLockMode.WriteOnly,
            bmp.PixelFormat);
        frame.CopyPixels(Int32Rect.Empty, data.Scan0, data.Height * data.Stride, data.Stride);
        bmp.UnlockBits(data);
        _trayIcon = Icon.FromHandle(bmp.GetHicon());
        return _trayIcon;
    }
}
