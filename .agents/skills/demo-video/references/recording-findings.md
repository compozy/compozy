# Recording findings — measured, do not re-derive

Every entry below was measured during the original pipeline build (Compozy web app, Playwright 1.62, Chromium, macOS). They cost most of a session to establish. Treat them as ground truth until a new measurement — not a hunch — contradicts one.

## Capture

1. **`context.recordVideo` has a hard quality ceiling.** Playwright's `videoRecorder.ts` hardcodes `-r 25 -c:v vp8 -b:v 1M -deadline realtime -speed 8`. The 1 Mbps is independent of resolution — at 4K it produced 430 kbps and visible blocking. Not configurable. This is why the recorder uses `page.screencast.start({onFrame})` and encodes separately.

2. **Capture fps scales inversely with the *rendering surface* size**, not the output size. Measured: 2560×1440 surface → ~72fps · 2880×1620 → ~57fps · 3840×2160 → ~25fps (all idle-page rates; under animation load expect ~25-30fps at 2880×1620). **3200×1800 delivers zero frames** — its fractional device scale factor (1.667 over a 1920×1080 viewport) breaks the screencast; 1.5 and 2 work. Keep viewport at 1920×1080 for a comfortable UI size and raise quality via the surface, never via CSS zoom.

3. **`--disable-frame-rate-limit` / `--disable-gpu-vsync` make capture worse** — 25fps → 13fps. The compositor stops pacing itself and JPEG encoding becomes the bottleneck.

4. **CSS `zoom` breaks pointer hit-testing.** `boundingBox()` returns layout px while `page.mouse` uses visual px (ratio = the zoom factor). Even with corrected coordinates, clicks don't register. Dead end — use a bigger surface instead.

5. **`deviceScaleFactor` alone does not raise the capture surface.** Without `--force-device-scale-factor`, Chromium places a 1x surface in the top-left of the larger canvas and pads the rest. The launch flag makes it real pixels. The recorder passes both.

## Cursor

6. **Cursor smoothness is capture-fps-bound, so gesture speed matters.** Each `page.mouse.move` costs 50-70ms over CDP — a 300ms gesture gets only 5-8 real positions. The overlay therefore animates **in-page with rAF**, and the real mouse moves once per gesture (at 80% of the span, so hover state is live on arrival). Speed is constant px/s: a capped-duration formula makes long moves ~5× faster than short ones and stops reading as one hand.

7. **Playwright ≥1.62 ships built-in demo helpers**, unused so far: `page.screencast.showActions({cursor:'pointer'})`, `showChapter(title, {description})`, `showOverlay(html)`. `showChapter` gives free narration cards if a demo wants them.

## Quality math (zoom vs upscale)

At peak zoom the encoder crops `surface_width / zoom` pixels and scales to `OUT_W`. Upscale factor = `OUT_W × zoom / surface_width`. From a 2880 surface at zoom 1.5 to a 2560 output that is 1.33× — slightly soft. Options, in order of preference: cap peak zoom at ~1.25 (near-native), output 1920×1080 (1.5 becomes native), or accept the softness. Raising the surface costs fps (finding 2).

## The abandoned spike — deterministic capture

Goal was rendering frames under `Emulation.setVirtualTimePolicy` (virtual clock) to decouple resolution from fps. Status: **abandoned; do not resume casually.**

- `HeadlessExperimental.beginFrame` hangs in Playwright's Chromium and crashes chrome-headless-shell 147-149 — the domain is deprecated.
- Virtual time itself works (21.3ms/frame stepped), but `Page.captureScreenshot` stalls at the same frame in both a Playwright-API build and a raw-CDP build, so the Playwright-API hypothesis is disproven.
- The clock cannot be frozen before navigation — page load starves and `goto` times out. Boot on real time, engage virtual time only for the filmed stretch.
- `captureBeyondViewport` (even `false`) puts Chrome on a surface-resizing path costing ~18s/frame at 4K — omit the key entirely.
- Three plausible diagnoses (CSS transition, `captureBeyondViewport`, Playwright APIs) were each disproven by isolated probes. Any new hypothesis needs an isolated probe **that includes a screenshot capture** — a probe without capture proves nothing here.
- The untried angle, if someone is determined: drive `Page.startScreencast` one-frame-per-tick instead of `captureScreenshot`.

The prize was only removing a 1.33× upscale at peak zoom. The realtime pipeline is the shipped answer.
