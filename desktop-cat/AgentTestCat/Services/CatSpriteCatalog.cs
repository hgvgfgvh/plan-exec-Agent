using System.Windows;
using System.Windows.Media;
using System.Windows.Media.Imaging;

namespace AgentTestCat.Services;

/// <summary>加载桌宠精灵图，黑底转透明并缓存。</summary>
public static class CatSpriteCatalog
{
    private static readonly Dictionary<string, ImageSource> Cache = new(StringComparer.OrdinalIgnoreCase);

    public static ImageSource Idle => Get("cat_idle.png");
    public static ImageSource Blink => Get("cat_blink.png");
    public static ImageSource Working => Get("cat_working.png");
    public static ImageSource Happy => Get("cat_happy.png");
    public static ImageSource Sleep => Get("cat_sleep.png");
    public static ImageSource Sad => Get("cat_sad.png");
    public static ImageSource Confused => Get("cat_confused.png");
    public static ImageSource Upload => Get("cat_upload.png");

    public static ImageSource Get(PetMood mood, bool blinking = false) => mood switch
    {
        PetMood.Working => Working,
        PetMood.Resting => Sleep,
        PetMood.Happy => Happy,
        PetMood.Sad => Sad,
        PetMood.Confused => Confused,
        PetMood.Uploading => Upload,
        PetMood.Idle when blinking => Blink,
        _ => Idle
    };

    public static ImageSource Get(string fileName)
    {
        lock (Cache)
        {
            if (Cache.TryGetValue(fileName, out var cached))
                return cached;

            var uri = new Uri($"pack://application:,,,/Assets/cat/{fileName}", UriKind.Absolute);
            var decoder = BitmapDecoder.Create(uri, BitmapCreateOptions.None, BitmapCacheOption.OnLoad);
            var frame = decoder.Frames[0];

            // 统一转为标准 BGRA，避免索引色/预乘格式导致 CopyPixels 读出灰度或错色
            var bgra = new FormatConvertedBitmap(frame, PixelFormats.Bgra32, null, 0);
            bgra.Freeze();

            var keyed = KeyOutBackground(bgra);
            var display = new FormatConvertedBitmap(keyed, PixelFormats.Pbgra32, null, 0);
            display.Freeze();
            Cache[fileName] = display;
            return display;
        }
    }

    private static BitmapSource KeyOutBackground(BitmapSource source)
    {
        var width = source.PixelWidth;
        var height = source.PixelHeight;
        var stride = width * 4;
        var pixels = new byte[height * stride];
        source.CopyPixels(pixels, stride, 0);

        var bg = new bool[width * height];
        var queue = new Queue<(int x, int y)>();

        void TrySeed(int x, int y)
        {
            if (x < 0 || y < 0 || x >= width || y >= height) return;
            var i = y * width + x;
            if (bg[i] || !IsRemovableBackground(pixels, i * 4)) return;
            bg[i] = true;
            queue.Enqueue((x, y));
        }

        for (var x = 0; x < width; x++)
        {
            TrySeed(x, 0);
            TrySeed(x, height - 1);
        }

        for (var y = 0; y < height; y++)
        {
            TrySeed(0, y);
            TrySeed(width - 1, y);
        }

        while (queue.Count > 0)
        {
            var (x, y) = queue.Dequeue();
            TrySeed(x - 1, y);
            TrySeed(x + 1, y);
            TrySeed(x, y - 1);
            TrySeed(x, y + 1);
        }

        for (var i = 0; i < bg.Length; i++)
        {
            if (!bg[i]) continue;
            var p = i * 4;
            pixels[p + 3] = 0;
        }

        var result = BitmapSource.Create(width, height, source.DpiX, source.DpiY,
            PixelFormats.Bgra32, null, pixels, stride);
        return result;
    }

    /// <summary>仅抠纯黑/近黑背景，保留棕色等有色暗部。</summary>
    private static bool IsRemovableBackground(byte[] pixels, int p)
    {
        var b = pixels[p];
        var g = pixels[p + 1];
        var r = pixels[p + 2];
        if (r > 36 || g > 36 || b > 36) return false;

        var max = Math.Max(r, Math.Max(g, b));
        var min = Math.Min(r, Math.Min(g, b));
        // 有色彩倾向的暗像素（如深棕条纹）不当作背景
        if (max - min > 10) return false;
        return true;
    }
}
