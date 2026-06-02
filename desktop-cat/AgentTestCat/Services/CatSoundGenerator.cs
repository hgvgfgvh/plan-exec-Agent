using System.IO;

namespace AgentTestCat.Services;

/// <summary>生成短促「喵」声 WAV（无外部依赖，CC0）。</summary>
public static class CatSoundGenerator
{
    private const int SampleRate = 22050;

    public static byte[] CreateMeowWav(double durationSec = 0.38)
    {
        var sampleCount = (int)(SampleRate * durationSec);
        var data = new short[sampleCount];
        for (var i = 0; i < sampleCount; i++)
        {
            var t = i / (double)SampleRate;
            var env = Envelope(t, durationSec);
            // 喵：中频上扬再下落 + 轻微谐波
            var f0 = 420 + 520 * Math.Sin(Math.PI * Math.Min(1, t / (durationSec * 0.45)));
            var wave = Math.Sin(2 * Math.PI * f0 * t) * 0.7
                       + Math.Sin(2 * Math.PI * f0 * 1.8 * t) * 0.22
                       + Math.Sin(2 * Math.PI * f0 * 0.5 * t) * 0.12;
            data[i] = (short)Math.Clamp(wave * env * 28000, -32000, 32000);
        }
        return WrapWav(data);
    }

    public static byte[] CreatePurrWav(double durationSec = 0.25)
    {
        var sampleCount = (int)(SampleRate * durationSec);
        var data = new short[sampleCount];
        for (var i = 0; i < sampleCount; i++)
        {
            var t = i / (double)SampleRate;
            var env = Math.Min(1, t * 20) * Math.Min(1, (durationSec - t) * 12);
            var mod = 0.5 + 0.5 * Math.Sin(2 * Math.PI * 24 * t);
            var wave = Math.Sin(2 * Math.PI * 110 * t) * mod;
            data[i] = (short)Math.Clamp(wave * env * 12000, -32000, 32000);
        }
        return WrapWav(data);
    }

    public static void EnsureSoundFiles(string dir)
    {
        Directory.CreateDirectory(dir);
        WriteIfMissing(Path.Combine(dir, "meow.wav"), CreateMeowWav());
        WriteIfMissing(Path.Combine(dir, "meow_short.wav"), CreateMeowWav(0.22));
        WriteIfMissing(Path.Combine(dir, "purr.wav"), CreatePurrWav());
    }

    private static void WriteIfMissing(string path, byte[] wav)
    {
        if (File.Exists(path) && new FileInfo(path).Length > 500) return;
        File.WriteAllBytes(path, wav);
    }

    private static double Envelope(double t, double duration)
    {
        var attack = Math.Min(1, t / 0.03);
        var release = Math.Min(1, (duration - t) / 0.08);
        return attack * release;
    }

    private static byte[] WrapWav(short[] pcm)
    {
        using var ms = new MemoryStream();
        using var w = new BinaryWriter(ms);
        var dataSize = pcm.Length * 2;
        w.Write("RIFF"u8);
        w.Write(36 + dataSize);
        w.Write("WAVE"u8);
        w.Write("fmt "u8);
        w.Write(16);
        w.Write((short)1);
        w.Write((short)1);
        w.Write(SampleRate);
        w.Write(SampleRate * 2);
        w.Write((short)2);
        w.Write((short)16);
        w.Write("data"u8);
        w.Write(dataSize);
        foreach (var s in pcm)
            w.Write(s);
        return ms.ToArray();
    }
}
