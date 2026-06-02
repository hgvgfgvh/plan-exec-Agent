using System.IO;
using System.Media;
using System.Windows.Threading;

namespace AgentTestCat.Services;

public sealed class CatSoundService
{
    private readonly AppSettings _settings;
    private readonly string _soundDir;
    private readonly Dispatcher _dispatcher;
    private readonly Random _rng = new();

    public CatSoundService(AppSettings settings, Dispatcher dispatcher)
    {
        _settings = settings;
        _dispatcher = dispatcher;
        _soundDir = Path.Combine(AppSettings.ConfigDir, "sounds");
        CatSoundGenerator.EnsureSoundFiles(_soundDir);
    }

    public void PlayMeow() => Play("meow.wav");

    public void PlayMeowShort() => Play("meow_short.wav");

    public void PlayPurr() => Play("purr.wav");

    public void PlayRandomMeow() =>
        Play(_rng.Next(2) == 0 ? "meow_short.wav" : "meow.wav");

    private void Play(string fileName)
    {
        if (!_settings.EnableSound) return;
        var path = Path.Combine(_soundDir, fileName);
        if (!File.Exists(path)) return;

        _dispatcher.BeginInvoke(() =>
        {
            try
            {
                using var player = new SoundPlayer(path);
                player.Play();
            }
            catch
            {
                /* ignore audio errors */
            }
        }, DispatcherPriority.Background);
    }
}
