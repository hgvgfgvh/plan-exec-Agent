using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Threading;

namespace AgentTestCat.Services;

/// <summary>桌宠精灵状态、眨眼、呼吸、戳猫与特效。</summary>
public sealed class CatAnimator
{
    private static readonly TimeSpan RestAfter = TimeSpan.FromMinutes(5);
    private static readonly TimeSpan TransientMood = TimeSpan.FromMilliseconds(1600);

    private readonly System.Windows.Controls.Image _image;
    private readonly ScaleTransform _scale;
    private readonly TranslateTransform _translate;
    private readonly Canvas _fxCanvas;
    private readonly Border _bubble;
    private readonly TextBlock _bubbleText;
    private readonly DispatcherTimer _tick;

    private PetMood _mood = PetMood.Idle;
    private PetMood _persistentMood = PetMood.Idle;
    private bool _blinking;
    private bool _transient;
    private int _tickFrame;
    private int _pokeCount;
    private DateTime _lastActivity = DateTime.UtcNow;
    private DateTime _nextBlink = DateTime.UtcNow.AddSeconds(2.5);
    private DateTime _lastZzz = DateTime.MinValue;
    private DispatcherTimer? _transientTimer;

    public CatAnimator(
        System.Windows.Controls.Image image,
        ScaleTransform scale,
        TranslateTransform translate,
        Canvas fxCanvas,
        Border bubble,
        TextBlock bubbleText,
        DispatcherTimer tickTimer)
    {
        _image = image;
        _scale = scale;
        _translate = translate;
        _fxCanvas = fxCanvas;
        _bubble = bubble;
        _bubbleText = bubbleText;
        _tick = tickTimer;
        _tick.Tick += OnTick;
        ApplySprite(false);
    }

    public void SetMood(PetMood mood)
    {
        if (IsTransient(mood))
        {
            if (mood == PetMood.Happy || mood == PetMood.Sad)
                _persistentMood = PetMood.Idle;
            _transient = true;
            _mood = mood;
            ApplySprite(false);
            PlayMoodFx(mood);
            RestartTransientTimer();
            return;
        }

        _transient = false;
        _transientTimer?.Stop();
        _persistentMood = mood;
        _mood = mood;
        _lastActivity = DateTime.UtcNow;
        ApplySprite(false);
    }

    public void NotifyActivity() => _lastActivity = DateTime.UtcNow;

    public void OnPoke(int clickCount)
    {
        NotifyActivity();
        _pokeCount++;
        if (_persistentMood == PetMood.Resting)
        {
            _persistentMood = PetMood.Idle;
            _mood = PetMood.Idle;
            ApplySprite(false);
        }

        var spam = _pokeCount >= 4;
        ShowBubble(CatQuips.RandomPoke(spam));
        _scale.ScaleX = 1.06;
        _scale.ScaleY = 0.94;
        CatFxOverlay.SpawnHeart(_fxCanvas, 64, 18);

        if (spam)
            _pokeCount = 0;
    }

    private void OnTick(object? sender, EventArgs e)
    {
        _tickFrame++;

        if (!_transient && _persistentMood == PetMood.Idle &&
            DateTime.UtcNow - _lastActivity > RestAfter)
        {
            _mood = PetMood.Resting;
        }
        else if (!_transient && _mood == PetMood.Resting &&
                 DateTime.UtcNow - _lastActivity <= RestAfter)
        {
            _mood = PetMood.Idle;
        }

        UpdateBlink();
        UpdateBobbing();

        if (_mood == PetMood.Resting && DateTime.UtcNow - _lastZzz > TimeSpan.FromSeconds(2.8))
        {
            _lastZzz = DateTime.UtcNow;
            CatFxOverlay.SpawnZzz(_fxCanvas, 72 + _tickFrame % 3 * 4, 6);
        }
    }

    private void UpdateBlink()
    {
        if (_mood is not (PetMood.Idle or PetMood.Resting))
        {
            _blinking = false;
            return;
        }

        if (_blinking)
        {
            if (DateTime.UtcNow >= _nextBlink)
            {
                _blinking = false;
                _nextBlink = DateTime.UtcNow.AddSeconds(2.5 + Random.Shared.NextDouble() * 2.5);
                ApplySprite(false);
            }
            return;
        }

        if (DateTime.UtcNow >= _nextBlink)
        {
            _blinking = true;
            _nextBlink = DateTime.UtcNow.AddMilliseconds(140);
            ApplySprite(true);
        }
    }

    private void UpdateBobbing()
    {
        if (_mood == PetMood.Working)
        {
            var wobble = Math.Sin(_tickFrame * 0.35) * 2.5;
            _translate.X = wobble;
            _scale.ScaleX = 1 + Math.Sin(_tickFrame * 0.28) * 0.02;
            _scale.ScaleY = 1 - Math.Sin(_tickFrame * 0.28) * 0.02;
            return;
        }

        if (_mood == PetMood.Happy)
        {
            var bounce = Math.Abs(Math.Sin(_tickFrame * 0.5)) * 4;
            _translate.Y = -bounce;
            _translate.X = 0;
            return;
        }

        if (_mood is PetMood.Idle or PetMood.Resting)
        {
            var breath = Math.Sin(_tickFrame * 0.12) * 1.2;
            _translate.X = 0;
            _translate.Y = breath;
            _scale.ScaleX = 1 + Math.Sin(_tickFrame * 0.12) * 0.015;
            _scale.ScaleY = 1 - Math.Sin(_tickFrame * 0.12) * 0.015;
            return;
        }

        _translate.X = 0;
        _translate.Y = 0;
        _scale.ScaleX = 1;
        _scale.ScaleY = 1;
    }

    private void ApplySprite(bool blinkOverride)
    {
        _image.Source = CatSpriteCatalog.Get(_mood, blinkOverride && _mood is PetMood.Idle or PetMood.Resting);
    }

    private void PlayMoodFx(PetMood mood)
    {
        switch (mood)
        {
            case PetMood.Happy:
                CatFxOverlay.BurstHearts(_fxCanvas, 64, 20);
                break;
            case PetMood.Sad:
                CatFxOverlay.SpawnSweat(_fxCanvas, 88, 24);
                break;
            case PetMood.Confused:
                CatFxOverlay.SpawnQuestion(_fxCanvas, 70, 8);
                break;
        }
    }

    private void RestartTransientTimer()
    {
        _transientTimer ??= new DispatcherTimer { Interval = TransientMood };
        _transientTimer.Stop();
        _transientTimer.Tick -= TransientTick;
        _transientTimer.Tick += TransientTick;
        _transientTimer.Start();
    }

    private void TransientTick(object? sender, EventArgs e)
    {
        _transientTimer?.Stop();
        _transient = false;
        _mood = _persistentMood;
        ApplySprite(false);
    }

    private void ShowBubble(string text)
    {
        _bubbleText.Text = text;
        _bubble.Visibility = Visibility.Visible;

        var hide = new DispatcherTimer { Interval = TimeSpan.FromSeconds(2.4) };
        hide.Tick += (_, _) =>
        {
            hide.Stop();
            _bubble.Visibility = Visibility.Collapsed;
        };
        hide.Start();
    }

    private static bool IsTransient(PetMood mood) =>
        mood is PetMood.Happy or PetMood.Sad;
}
