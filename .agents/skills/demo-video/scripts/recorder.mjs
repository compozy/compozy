/**
 * demo-video recorder library — imported by per-demo flow files.
 *
 * Mutating: drives a real browser and writes frames + camera track to outDir.
 *
 * Only node:* imports on purpose: the flow file resolves `@playwright/test`
 * from its own project and hands `chromium` in, so this file works from the
 * skill directory without a node_modules of its own.
 *
 * Every numeric default below was measured, not guessed — the evidence lives
 * in references/recording-findings.md next to this skill.
 */
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

// The overlay is animated in-page with rAF instead of following mousemove:
// a CDP mouse.move costs ~50-70ms under high-res capture, which would give a
// 300ms gesture only 5-8 positions. rAF gives it one position per painted frame.
const CURSOR_SCRIPT = `
(() => {
  const install = () => {
    if (document.getElementById("__demo_cursor__")) return;
    const style = document.createElement("style");
    style.textContent = \`
      #__demo_cursor__ {
        position: fixed; left: 0; top: 0; width: 26px; height: 26px;
        margin: -13px 0 0 -13px; border-radius: 50%;
        background: rgba(255,255,255,.92);
        box-shadow: 0 0 0 1.5px rgba(0,0,0,.45), 0 6px 18px rgba(0,0,0,.55);
        z-index: 2147483647; pointer-events: none; opacity: 0;
        transition: opacity .25s ease, transform .12s ease;
        will-change: transform;
      }
      #__demo_cursor__.__down__ { transform: scale(.72); }
      .__demo_ripple__ {
        position: fixed; width: 26px; height: 26px; margin: -13px 0 0 -13px;
        border-radius: 50%; border: 2.5px solid rgba(255,255,255,.85);
        z-index: 2147483646; pointer-events: none;
        animation: __demo_ripple__ .65s cubic-bezier(.22,.61,.36,1) forwards;
      }
      @keyframes __demo_ripple__ {
        from { transform: scale(1); opacity: .9; }
        to   { transform: scale(3.2); opacity: 0; }
      }
    \`;
    document.head.appendChild(style);

    const dot = document.createElement("div");
    dot.id = "__demo_cursor__";
    document.body.appendChild(dot);

    const place = (x, y) => {
      dot.style.left = x + "px";
      dot.style.top = y + "px";
    };

    window.__demoCursor = {
      set(x, y) {
        place(x, y);
        dot.style.opacity = "1";
      },
      glide(from, to, ms, bow, nx, ny) {
        dot.style.opacity = "1";
        const ease = t => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);
        return new Promise(resolve => {
          const start = performance.now();
          const step = now => {
            const p = Math.min(1, (now - start) / ms);
            const t = ease(p);
            const arc = Math.sin(p * Math.PI) * bow;
            place(
              from.x + (to.x - from.x) * t + nx * arc,
              from.y + (to.y - from.y) * t + ny * arc
            );
            if (p < 1) requestAnimationFrame(step);
            else resolve();
          };
          requestAnimationFrame(step);
        });
      },
    };

    document.addEventListener("mousedown", e => {
      dot.classList.add("__down__");
      const r = document.createElement("div");
      r.className = "__demo_ripple__";
      r.style.left = e.clientX + "px";
      r.style.top = e.clientY + "px";
      document.body.appendChild(r);
      setTimeout(() => r.remove(), 700);
    }, { capture: true, passive: true });

    document.addEventListener("mouseup", () => dot.classList.remove("__down__"), {
      capture: true, passive: true,
    });
  };
  if (document.body) install();
  else document.addEventListener("DOMContentLoaded", install, { once: true });
})();
`;

const sleep = ms => new Promise(r => setTimeout(r, ms));

/**
 * Boot a browser sized for capture and return the demo session handle.
 *
 * `chromium` is the export of the host project's `@playwright/test`.
 * `surface` must be an integer-or-half multiple of `viewport` (scale 1.5 or 2):
 * fractional device scale factors like 1.667 deliver zero frames.
 */
export async function openDemo(chromium, {
  url,
  outDir,
  viewport = { width: 1920, height: 1080 },
  surface = { width: 2880, height: 1620 },
  colorScheme = "dark",
  readySelector = null,
  settleMs = 1200,
  cursorSpeed = 850,
  quality = 100,
  launchArgs = [],
} = {}) {
  if (!url) throw new Error("openDemo: `url` is required");
  if (!outDir) throw new Error("openDemo: `outDir` is required");

  const scale = surface.width / viewport.width;
  if (Math.abs(surface.height / viewport.height - scale) > 1e-6) {
    throw new Error(
      `surface ${surface.width}x${surface.height} is not the same aspect as viewport ` +
        `${viewport.width}x${viewport.height}`
    );
  }
  if (![1, 1.5, 2].includes(scale)) {
    console.error(
      `warning: device scale factor ${scale.toFixed(3)} is unproven — measured-good values ` +
        `are 1.5 and 2; 1.667 (3200x1800 over 1920x1080) delivers zero frames`
    );
  }

  const frameDir = path.join(outDir, "frames");
  mkdirSync(frameDir, { recursive: true });

  const browser = await chromium.launch({
    headless: true,
    // --disable-frame-rate-limit / --disable-gpu-vsync were measured and HALVE
    // the delivered frame rate (25fps -> 13fps): the compositor stops pacing
    // itself and JPEG encoding becomes the bottleneck. Leave them off.
    // --force-device-scale-factor is what makes the capture surface real pixels;
    // deviceScaleFactor alone pads a 1x surface into the corner of the frame.
    args: [`--force-device-scale-factor=${scale}`, "--high-dpi-support=1", ...launchArgs],
  });
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: scale,
    colorScheme,
    reducedMotion: "no-preference",
  });
  await context.addInitScript(CURSOR_SCRIPT);
  const page = await context.newPage();

  await page.goto(url, { waitUntil: "networkidle" });
  if (readySelector) await page.waitForSelector(readySelector);
  if (settleMs > 0) await page.waitForTimeout(settleMs);

  // ------------------------------------------------------------ mouse driver
  // Constant px/s, not capped duration: a duration cap makes long moves ~5x
  // faster than short ones and the pace stops reading as one hand. Capture
  // tops out around 25fps, so speed controls how far the cursor jumps between
  // frames: 1350px/s leaves 54px gaps (stutter), 850px/s leaves 34px (glide).
  const MIN_MOVE_MS = 180;
  const MAX_MOVE_MS = 1500;
  const pos = { x: viewport.width / 2, y: viewport.height - 40 };
  const timings = [];

  async function moveTo(x, y, { speed = cursorSpeed } = {}) {
    const from = { ...pos };
    const dist = Math.hypot(x - from.x, y - from.y);
    if (dist < 1) return;

    const ms = Math.max(MIN_MOVE_MS, Math.min(MAX_MOVE_MS, (dist / speed) * 1000));
    // slight arc so the path is not a dead-straight line
    const nx = -(y - from.y) / dist;
    const ny = (x - from.x) / dist;
    const bow = Math.min(viewport.width * 0.031, dist * 0.09);

    // The visible glide runs in-page at rAF rate; the real pointer only lands
    // near the end (80% of the span) so hover/tooltip state is live on arrival.
    // The glide is not awaited — that would add a second CDP hop (~60ms) of
    // dead time to every gesture. Node sleeps the same span instead.
    const start = performance.now();
    page
      .evaluate(
        ([from, to, ms, bow, nx, ny]) => {
          window.__demoCursor.glide(from, to, ms, bow, nx, ny);
        },
        [from, { x, y }, ms, bow, nx, ny]
      )
      .catch(() => {});
    await sleep(ms * 0.8);
    await page.mouse.move(x, y);

    timings.push({
      dist: Math.round(dist),
      target: Math.round(ms),
      actual: Math.round(performance.now() - start),
    });
    pos.x = x;
    pos.y = y;
  }

  async function moveToLocator(locator, opts) {
    await locator.waitFor({ state: "visible" });
    const box = await locator.boundingBox();
    if (!box) throw new Error("moveToLocator: locator has no box");
    await moveTo(box.x + box.width / 2, box.y + box.height / 2, opts);
  }

  async function click({ dwell = 260 } = {}) {
    await sleep(dwell);
    await page.mouse.down();
    await sleep(90);
    await page.mouse.up();
  }

  const clickLocator = async (locator, opts) => {
    await moveToLocator(locator, opts);
    await click(opts);
  };

  // ------------------------------------------------------------ camera beats
  // The flow knows what matters and when, so it emits the camera track inline
  // instead of the encoder guessing later. Rects are CSS px; encode.mjs scales
  // them into frame space. `ease` is how long the glide into this framing takes.
  const beats = [];
  const nowSec = () => Date.now() / 1000;

  async function beat(locator, zoom, { ease = 750, label = "" } = {}) {
    let rect = null;
    if (locator) {
      rect = await locator.boundingBox();
      if (!rect) throw new Error(`beat "${label}": locator has no box`);
    }
    beats.push({ t: nowSec(), zoom, rect, ease, label });
  }

  /** Place the cursor at a viewport fraction and fade the overlay in. */
  async function park(fx = 0.5, fy = 0.78) {
    const x = Math.round(viewport.width * fx);
    const y = Math.round(viewport.height * fy);
    await page.mouse.move(x, y);
    await page.evaluate(([px, py]) => window.__demoCursor.set(px, py), [x, y]);
    pos.x = x;
    pos.y = y;
  }

  // ------------------------------------------------------------ capture
  // Raw JPEG frames instead of context.recordVideo: Playwright's built-in
  // writer (videoRecorder.ts) is hardcoded to `-r 25 -c:v vp8 -b:v 1M
  // -deadline realtime -speed 8` — a 1 Mbps ceiling regardless of resolution.
  const frames = [];
  let capturing = false;

  async function startCapture() {
    if (capturing) return;
    capturing = true;
    await page.screencast.start({
      quality,
      size: surface,
      onFrame: ({ data, timestamp }) => {
        const file = path.join(frameDir, `f-${String(frames.length).padStart(5, "0")}.jpg`);
        writeFileSync(file, data);
        frames.push({ file, timestamp });
      },
    });
  }

  async function finish({ tailMs = 0 } = {}) {
    if (tailMs > 0) await page.waitForTimeout(tailMs);
    if (capturing) await page.screencast.stop();
    await context.close();
    await browser.close();

    if (frames.length === 0) {
      throw new Error(
        "finish: zero frames captured — check the device scale factor (fractional " +
          "values deliver nothing) and that startCapture() ran before the choreography"
      );
    }

    // Frame timestamps are irregular (the compositor only presents on change),
    // so the concat list carries an explicit per-frame duration; the encoder
    // resamples to a constant rate. A naive `fps=` filter would duplicate
    // frames at the wrong instants and read as judder.
    const unit = frames[1] && frames[1].timestamp - frames[0].timestamp < 1 ? 1 : 1 / 1000;
    const times = frames.map(f => f.timestamp * unit);
    const span = times.at(-1) - times[0];
    const gaps = times.slice(1).map((t, i) => t - times[i]).sort((a, b) => a - b);
    const medianGap = gaps[gaps.length >> 1] ?? 0.04;
    const fps = frames.length / Math.max(span, 1e-6);

    const lines = [];
    for (let i = 0; i < frames.length; i++) {
      const dur = i < frames.length - 1 ? times[i + 1] - times[i] : medianGap;
      lines.push(`file '${path.basename(frames[i].file)}'`, `duration ${dur.toFixed(6)}`);
    }
    lines.push(`file '${path.basename(frames.at(-1).file)}'`);
    writeFileSync(path.join(frameDir, "frames.txt"), lines.join("\n"));

    const track = {
      frameSize: surface,
      viewport,
      scale,
      t0: times[0],
      frames: times.map((t, i) => ({ file: path.basename(frames[i].file), t: t - times[0] })),
      beats: beats.map(b => ({
        t: b.t - times[0],
        zoom: b.zoom,
        ease: b.ease,
        label: b.label,
        rect: b.rect && {
          x: b.rect.x * scale,
          y: b.rect.y * scale,
          width: b.rect.width * scale,
          height: b.rect.height * scale,
        },
      })),
    };
    writeFileSync(path.join(outDir, "camera.json"), JSON.stringify(track, null, 2));

    console.log(
      `captured ${frames.length} frames over ${span.toFixed(2)}s = ${fps.toFixed(1)} fps ` +
        `(median gap ${(medianGap * 1000).toFixed(1)}ms)`
    );
    for (const t of timings) {
      console.log(
        `  move ${String(t.dist).padStart(5)}px  target ${String(t.target).padStart(4)}ms  ` +
          `actual ${String(t.actual).padStart(4)}ms`
      );
    }
    for (const b of track.beats) {
      console.log(`  beat ${b.t.toFixed(2)}s  ${String(b.zoom).padStart(4)}x  ${b.label}`);
    }
    return { frames: frames.length, fps, span, camera: track };
  }

  return {
    page,
    context,
    browser,
    viewport,
    surface,
    mouse: { moveTo, moveToLocator, click, clickLocator, pos },
    park,
    beat,
    startCapture,
    finish,
  };
}
