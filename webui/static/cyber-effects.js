/**
 * Cyberpunk UI — 鼠标跟随、视差、间歇故障触发
 */
(function (global) {
  const prefersReduced =
      global.matchMedia &&
      global.matchMedia("(prefers-reduced-motion: reduce)").matches;

  let raf = 0;
  let mx = 0;
  let my = 0;
  let targetMx = 0;
  let targetMy = 0;
  let bound = false;

  function lerp(a, b, t) {
    return a + (b - a) * t;
  }

  function setGlow(x, y) {
    const glow = document.getElementById("cyber-cursor-glow");
    if (!glow) return;
    glow.style.setProperty("--gx", x + "px");
    glow.style.setProperty("--gy", y + "px");
  }

  function setGrid(x, y) {
    const bg = document.getElementById("cyber-bg");
    if (!bg) return;
    const w = global.innerWidth || 1;
    const h = global.innerHeight || 1;
    const px = ((x / w) * 100).toFixed(2) + "%";
    const py = ((y / h) * 100).toFixed(2) + "%";
    bg.style.setProperty("--spot-x", px);
    bg.style.setProperty("--spot-y", py);
  }

  function tiltCards(x, y) {
    const cards = document.querySelectorAll("[data-cyber-tilt]");
    const w = global.innerWidth || 1;
    const h = global.innerHeight || 1;
    const nx = (x / w - 0.5) * 2;
    const ny = (y / h - 0.5) * 2;
    cards.forEach((el) => {
      const rect = el.getBoundingClientRect();
      const cx = rect.left + rect.width / 2;
      const cy = rect.top + rect.height / 2;
      const dx = (x - cx) / Math.max(rect.width, 1);
      const dy = (y - cy) / Math.max(rect.height, 1);
      const rotY = Math.max(-8, Math.min(8, dx * 6 + nx * 2));
      const rotX = Math.max(-6, Math.min(6, -dy * 5 - ny * 2));
      el.style.setProperty("--tilt-x", rotX.toFixed(2) + "deg");
      el.style.setProperty("--tilt-y", rotY.toFixed(2) + "deg");
    });
  }

  function tick() {
    raf = 0;
    mx = lerp(mx, targetMx, 0.14);
    my = lerp(my, targetMy, 0.14);
    setGlow(mx, my);
    setGrid(mx, my);
    tiltCards(mx, my);
    if (Math.abs(mx - targetMx) > 0.5 || Math.abs(my - targetMy) > 0.5) {
      raf = global.requestAnimationFrame(tick);
    }
  }

  function scheduleTick() {
    if (!raf) raf = global.requestAnimationFrame(tick);
  }

  function onPointerMove(e) {
    targetMx = e.clientX;
    targetMy = e.clientY;
    scheduleTick();
  }

  function triggerGlitch(el) {
    if (!el || prefersReduced) return;
    el.classList.remove("glitch-hit");
    void el.offsetWidth;
    el.classList.add("glitch-hit");
    global.setTimeout(() => el.classList.remove("glitch-hit"), 420);
    const title = el.querySelector(".glitch-text");
    if (title && title !== el) {
      title.classList.remove("glitch-hit");
      void title.offsetWidth;
      title.classList.add("glitch-hit");
      global.setTimeout(() => title.classList.remove("glitch-hit"), 420);
    }
  }

  function bindInteractiveGlitch() {
    const sel =
        ".btn-primary, .btn-send, .btn-ghost, .btn-attach, .login-brand h1, .top-bar-title, .welcome h2";
    document.querySelectorAll(sel).forEach((el) => {
      el.addEventListener("mouseenter", () => triggerGlitch(el));
    });
  }

  function bindMessageGlitchObserver() {
    const thread = document.getElementById("thread");
    if (!thread || prefersReduced) return;
    const obs = new MutationObserver((mutations) => {
      for (const m of mutations) {
        for (const node of m.addedNodes) {
          if (node.nodeType !== 1) continue;
          if (node.classList && node.classList.contains("message")) {
            global.setTimeout(() => triggerGlitch(node), 40);
          }
        }
      }
    });
    obs.observe(thread, { childList: true });
  }

  function startAmbientGlitch() {
    if (prefersReduced) return;
    const titles = document.querySelectorAll(".glitch-text");
    global.setInterval(() => {
      const el = titles[Math.floor(Math.random() * titles.length)];
      if (el) triggerGlitch(el);
    }, 5200 + Math.random() * 4000);
  }

  function bind() {
    if (bound) return;
    bound = true;
    document.body.classList.add("cyber-ui");
    document.addEventListener("pointermove", onPointerMove, { passive: true });
    bindInteractiveGlitch();
    bindMessageGlitchObserver();
    startAmbientGlitch();
    scheduleTick();
  }

  function init() {
    bind();
  }

  global.CyberFx = { init, triggerGlitch };
})(window);
