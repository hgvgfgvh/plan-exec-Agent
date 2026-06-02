/**
 * 诡谲符号：可识别受难像素底 · 旋转四面体笼裹纺锤之眼 · 像素故障
 */
(function (global) {
  const W = 240;
  const H = 320;
  const PX = 2;

  const surfaces = [];
  let rafId = 0;
  let frame = 0;
  let pupilX = 0;
  let pupilY = 0;
  let targetPupilX = 0;
  let targetPupilY = 0;
  let mouseX = 0;
  let mouseY = 0;
  let glitchBurst = 0;

  const TETRA = [
    [0, 1.25, 0],
    [-1.05, -0.62, 0.82],
    [1.05, -0.62, 0.82],
    [0, -0.62, -0.82],
  ];
  const EDGES = [
    [0, 1],
    [0, 2],
    [0, 3],
    [1, 2],
    [2, 3],
    [3, 1],
  ];

  function rotY(p, a) {
    const c = Math.cos(a);
    const s = Math.sin(a);
    return [p[0] * c + p[2] * s, p[1], -p[0] * s + p[2] * c];
  }

  function rotX(p, a) {
    const c = Math.cos(a);
    const s = Math.sin(a);
    return [p[0], p[1] * c - p[2] * s, p[1] * s + p[2] * c];
  }

  function project(p, cx, cy, scale) {
    const z = p[2] + 3.4;
    const f = scale / z;
    return [cx + p[0] * f, cy + p[1] * f, z];
  }

  function setPx(ctx, x, y, color, a) {
    const px = Math.floor(x / PX) * PX;
    const py = Math.floor(y / PX) * PX;
    ctx.fillStyle = color;
    ctx.globalAlpha = a == null ? 1 : a;
    ctx.fillRect(px, py, PX, PX);
    ctx.globalAlpha = 1;
  }

  function inSpindle(dx, dy, rx, ry) {
    if (rx <= 0 || ry <= 0) return false;
    return (dx * dx) / (rx * rx) + (dy * dy) / (ry * ry) <= 1;
  }

  function drawLinePx(ctx, x0, y0, x1, y1, color, alpha, thick) {
    const dx = x1 - x0;
    const dy = y1 - y0;
    const steps = Math.max(Math.abs(dx), Math.abs(dy)) / PX;
    if (steps < 1) {
      setPx(ctx, x0, y0, color, alpha);
      return;
    }
    for (let i = 0; i <= steps; i++) {
      const t = i / steps;
      const x = x0 + dx * t;
      const y = y0 + dy * t;
      setPx(ctx, x, y, color, alpha);
      if (thick) {
        setPx(ctx, x + PX, y, color, alpha * 0.7);
        setPx(ctx, x, y + PX, color, alpha * 0.7);
      }
    }
  }

  /** 经典耶稣受难 — 正面十字架上人形，一眼可辨 */
  function drawCrucifixion(ctx, cx, cy, glitch) {
    const gShift = glitch > 0.35 ? (Math.random() > 0.5 ? PX : -PX) : 0;

    for (let y = -110; y <= 110; y += PX) {
      for (let x = -90; x <= 90; x += PX) {
        const d = Math.abs(x) + Math.abs(y) * 0.3;
        if (d < 95) setPx(ctx, cx + x, cy + y - 20, "#120810", 0.35);
      }
    }

    const beamY = cy - 8;
    const vTop = cy - 98;
    const vBot = cy + 88;
    for (let y = vTop; y <= vBot; y += PX) {
      for (let dx = -5; dx <= 5; dx += PX) {
        const wood =
            Math.abs(dx) <= PX
                ? "#6a4030"
                : Math.abs(dx) <= 3
                  ? "#5a3428"
                  : "#4a2a20";
        setPx(ctx, cx + dx + gShift, y, wood, 0.82);
      }
    }

    const armY = cy - 28;
    for (let x = cx - 72; x <= cx + 72; x += PX) {
      for (let dy = -4; dy <= 4; dy += PX) {
        setPx(ctx, x + gShift, armY + dy, "#5a3428", 0.85);
      }
    }
    for (let dx = -7; dx <= 7; dx += PX) {
      for (let dy = -7; dy <= 7; dy += PX) {
        setPx(ctx, cx + dx, beamY + dy, "#4a2818", 0.9);
      }
    }

    const headY = cy - 58;
    for (let dy = -9; dy <= 9; dy += PX) {
      for (let dx = -8; dx <= 8; dx += PX) {
        if (dx * dx + dy * dy > 81) continue;
        setPx(ctx, cx + dx + gShift, headY + dy, "#d8a890", 0.88);
      }
    }
    for (let i = -10; i <= 10; i += PX * 2) {
      setPx(ctx, cx + i + gShift, headY - 10, "#3a2020", 0.75);
      setPx(ctx, cx + i * 0.8 + gShift, headY - 8, "#3a2020", 0.65);
    }
    for (let dy = -14; dy <= -10; dy += PX) {
      for (let dx = -12; dx <= 12; dx += PX) {
        if (Math.abs(dx) + Math.abs(dy + 12) * 0.8 < 14) {
          setPx(ctx, cx + dx, headY + dy, "#8a8070", 0.25);
        }
      }
    }

    for (let x = cx - 68; x <= cx - 38; x += PX) {
      for (let dy = -3; dy <= 3; dy += PX) {
        setPx(ctx, x + gShift, armY + dy, "#d0a088", 0.85);
      }
    }
    for (let x = cx + 38; x <= cx + 68; x += PX) {
      for (let dy = -3; dy <= 3; dy += PX) {
        setPx(ctx, x + gShift, armY + dy, "#d0a088", 0.85);
      }
    }
    setPx(ctx, cx - 70 + gShift, armY, "#5a3428", 0.9);
    setPx(ctx, cx + 70 + gShift, armY, "#5a3428", 0.9);

    for (let y = cy - 22; y <= cy + 42; y += PX) {
      for (let dx = -9; dx <= 9; dx += PX) {
        if (Math.abs(dx) > 6 && y > cy + 8) continue;
        setPx(ctx, cx + dx + gShift, y, "#c89880", 0.86);
      }
    }

    for (let y = cy + 12; y <= cy + 32; y += PX) {
      for (let dx = -14; dx <= 14; dx += PX) {
        const edge = Math.abs(dx) > 10 ? y > cy + 22 : false;
        if (!edge) setPx(ctx, cx + dx + gShift, y, "#e8e4dc", 0.82);
      }
    }

    for (let y = cy + 34; y <= cy + 72; y += PX) {
      for (let dx = -5; dx <= 5; dx += PX) {
        setPx(ctx, cx + dx * 0.55 + gShift, y, "#c09078", 0.84);
      }
    }
    setPx(ctx, cx - 8 + gShift, cy + 74, "#5a3428", 0.85);
    setPx(ctx, cx + 8 + gShift, cy + 74, "#5a3428", 0.85);

    for (let dx = -16; dx <= 16; dx += PX) {
      setPx(ctx, cx + dx + gShift, cy - 38, "#2a1818", 0.55);
    }
    for (let i = 0; i < 4; i++) {
      setPx(ctx, cx - 10 + i * 6 + gShift, cy - 36, "#c0b8a8", 0.5);
    }

    if (glitch > 0.25) {
      drawLinePx(ctx, cx - 80, cy - 60, cx + 80, cy - 60, "#ff2a6d", 0.15, false);
    }
  }

  function drawTetrahedronCage(ctx, cx, cy, t, glitch) {
    const scale = 88;
    const ay = t * 0.65;
    const ax = t * 0.42;
    const pts = TETRA.map((p) => {
      let v = rotY(p, ay);
      v = rotX(v, ax);
      return project(v, cx, cy, scale);
    });

    const colors = ["#00f0ff", "#ff2a6d", "#fcee0a", "#00f0ff", "#ff2a6d", "#fcee0a"];
    EDGES.forEach(([a, b], i) => {
      const p0 = pts[a];
      const p1 = pts[b];
      const off = glitch > 0.3 ? (i % 2 ? PX : -PX) : 0;
      drawLinePx(ctx, p0[0] + off, p0[1], p1[0] + off, p1[1], colors[i], 0.92, true);
      if (glitch > 0.2) {
        drawLinePx(ctx, p0[0] - off, p0[1], p1[0] - off, p1[1], "#ff2a6d", 0.28, false);
      }
    });

    pts.forEach((p, i) => {
      const sz = i === 0 ? 3 : 2;
      for (let dy = -sz; dy <= sz; dy += PX) {
        for (let dx = -sz; dx <= sz; dx += PX) {
          setPx(ctx, p[0] + dx, p[1] + dy, i === 0 ? "#fcee0a" : "#00f0ff", 0.95);
        }
      }
    });
  }

  /** 纺锤形（水平梭形）眼睛，瞳孔追鼠标 */
  function drawSpindleEye(ctx, cx, cy, px, py, glitch) {
    const rxW = 26;
    const ryW = 10;
    const rxI = 14;
    const ryI = 6;
    const rxP = 6;
    const ryP = 2.5;

    const ix = cx + px;
    const iy = cy + py;

    for (let dy = -ryW - PX; dy <= ryW + PX; dy += PX) {
      for (let dx = -rxW - PX; dx <= rxW + PX; dx += PX) {
        if (!inSpindle(dx, dy, rxW, ryW)) continue;
        if (inSpindle(dx, dy, rxI, ryI)) continue;
        setPx(ctx, cx + dx, cy + dy, "#14101c", 0.96);
      }
    }

    for (let dy = -ryI - PX; dy <= ryI + PX; dy += PX) {
      for (let dx = -rxI - PX; dx <= rxI + PX; dx += PX) {
        if (!inSpindle(dx, dy, rxI, ryI)) continue;
        if (inSpindle(dx - px, dy - py, rxP, ryP)) continue;
        const rim = inSpindle(dx, dy, rxI - 2, ryI - 1.5);
        setPx(ctx, cx + dx, cy + dy, rim ? "#00f0ff" : "#ff2a6d", 0.94);
      }
    }

    for (let dy = -ryP - PX; dy <= ryP + PX; dy += PX) {
      for (let dx = -rxP - PX; dx <= rxP + PX; dx += PX) {
        if (!inSpindle(dx, dy, rxP, ryP)) continue;
        setPx(ctx, ix + dx, iy + dy, "#030308", 1);
      }
    }

    if (glitch > 0.3) {
      setPx(ctx, ix + 8, iy - 4, "#fcee0a", 0.85);
      drawLinePx(ctx, cx - rxW, cy, cx + rxW, cy, "#ff2a6d", 0.2, false);
    }
  }

  function applySliceGlitch(ctx, canvas, amount) {
    if (amount <= 0) return;
    const slices = 2 + Math.floor(Math.random() * 4);
    for (let i = 0; i < slices; i++) {
      const sy = Math.floor(Math.random() * canvas.height * 0.88);
      const sh = PX * (2 + Math.floor(Math.random() * 5));
      const sx = (Math.random() > 0.5 ? 1 : -1) * (2 + Math.floor(Math.random() * 5));
      try {
        const img = ctx.getImageData(0, sy, canvas.width, sh);
        ctx.putImageData(img, sx, sy);
      } catch (_) {
        /* ignore */
      }
    }
  }

  function paint(entry) {
    const { ctx, canvas, sym, sctx } = entry;
    const cx = canvas.width / 2;
    const cy = canvas.height / 2 + 4;
    const t = frame * 0.02;
    const glitch = glitchBurst > 0 ? glitchBurst : Math.random() > 0.993 ? 0.6 : 0;

    ctx.fillStyle = "#050508";
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    drawCrucifixion(ctx, cx, cy, glitch);

    sctx.clearRect(0, 0, sym.width, sym.height);
    drawTetrahedronCage(sctx, cx, cy, t, glitch);
    drawSpindleEye(sctx, cx, cy, pupilX, pupilY, glitch);

    ctx.drawImage(sym, 0, 0);
    if (glitch > 0.18) {
      ctx.globalCompositeOperation = "screen";
      ctx.drawImage(sym, 3, 0);
      ctx.drawImage(sym, -2, 1);
      ctx.globalCompositeOperation = "source-over";
      applySliceGlitch(ctx, canvas, glitch);
    }

    ctx.fillStyle = "rgba(0, 240, 255, 0.18)";
    const scanY = (frame * 4) % canvas.height;
    ctx.fillRect(0, scanY, canvas.width, PX);

    if (glitchBurst > 0) glitchBurst -= 0.035;
  }

  function tick() {
    frame++;
    pupilX = pupilX * 0.8 + targetPupilX * 0.2;
    pupilY = pupilY * 0.8 + targetPupilY * 0.2;
    if (Math.random() < 0.007) glitchBurst = 0.7 + Math.random() * 0.3;
    for (const s of surfaces) paint(s);
    rafId = global.requestAnimationFrame(tick);
  }

  function updatePupilFromPointer(clientX, clientY) {
    let rect = null;
    for (const { canvas } of surfaces) {
      const r = canvas.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) {
        rect = r;
        break;
      }
    }
    if (!rect) return;
    const cx = rect.left + rect.width / 2;
    const cy = rect.top + rect.height / 2 + 4;
    const dx = clientX - cx;
    const dy = clientY - cy;
    const dist = Math.sqrt(dx * dx + dy * dy) || 1;
    const max = 9;
    targetPupilX = (dx / dist) * max;
    targetPupilY = (dy / dist) * max * 0.45;
  }

  function onPointerMove(e) {
    mouseX = e.clientX;
    mouseY = e.clientY;
    updatePupilFromPointer(e.clientX, e.clientY);
  }

  function init() {
    surfaces.length = 0;
    document.querySelectorAll(".ocular-sigil-canvas").forEach((canvas) => {
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      ctx.imageSmoothingEnabled = false;
      canvas.width = W;
      canvas.height = H;
      const sym = document.createElement("canvas");
      sym.width = W;
      sym.height = H;
      const sctx = sym.getContext("2d");
      if (!sctx) return;
      sctx.imageSmoothingEnabled = false;
      surfaces.push({ canvas, ctx, sym, sctx });
    });
    if (!surfaces.length) return;
    if (rafId) global.cancelAnimationFrame(rafId);
    rafId = global.requestAnimationFrame(tick);
    updatePupilFromPointer(mouseX || global.innerWidth / 2, mouseY || global.innerHeight / 2);
    if (!global.__ocularSigilBound) {
      global.__ocularSigilBound = true;
      document.addEventListener("pointermove", onPointerMove, { passive: true });
    }
  }

  global.OcularSigil = { init };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window);
