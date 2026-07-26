# Chrome DevTools Protocol capture flow

The `scripts/cap.mjs` helper drives Chrome via CDP because Chrome's built-in `--screenshot` flag hangs on Storybook iframes. This file documents why each step exists so that future edits do not regress the audit.

## Why CDP, not `chrome --headless --screenshot`

| Behaviour | `chrome --screenshot` | CDP via `cap.mjs` |
| --- | --- | --- |
| Wait for `load` event | implicit, unreliable | explicit `Page.loadEventFired()` |
| Wait for fonts | none — captures mid-FOUT | `Runtime.evaluate("document.fonts.ready")` |
| React/Storybook settle | impossible | explicit `--wait` sleep window |
| Reuses one Chrome instance for N shots | no (one Chrome per call) | yes (single `chrome.kill()` at end) |
| Concurrent calls compete for `--user-data-dir` | yes, deadlocks observed | n/a |

Empirically, the first audit ran ~50 storybook captures in ~2 minutes via CDP after `chrome --screenshot` produced zero PNGs in 30 s on the same URLs.

## Boot flags that matter

`scripts/cap.mjs` always passes:

```
--headless=new
--disable-gpu
--hide-scrollbars
--no-sandbox
--disable-dev-shm-usage
--mute-audio
--window-size=<W>,<H>
```

- `--headless=new` is the modern headless mode (not the deprecated `--headless=old`). Required for `document.fonts.ready` to resolve in some Chrome builds.
- `--disable-dev-shm-usage` prevents `/dev/shm`-bound crashes on Linux CI when the runner has a tight shm budget.
- `--hide-scrollbars` keeps captured viewports flush against the right edge.
- `--mute-audio` silences any story that plays audio (rare, but cheap insurance).

## Domain enablement order

```js
await Promise.all([Page.enable(), Network.enable(), Runtime.enable(), DOM.enable()]);
```

All four must be enabled before `setDeviceMetricsOverride` — `Page.enable()` alone is not enough for `loadEventFired` semantics, and `Runtime.enable()` is needed for `awaitPromise: true` on the fonts evaluator.

## Viewport determinism

```js
await Emulation.setDeviceMetricsOverride({
  width, height, deviceScaleFactor: 1, mobile: false,
});
```

`--window-size` alone is advisory — Chrome may pick a different inner size if the OS window manager interferes (macOS in particular). `setDeviceMetricsOverride` pins the layout viewport explicitly, so 1440 × 900 always means 1440 × 900.

## Per-shot sequence

For each `{ name, url }`:

1. `Page.navigate({ url })` — start navigation.
2. `Page.loadEventFired()` — block until the `load` event fires inside the iframe.
3. `sleep(waitMs)` — give React + Storybook + any Suspense boundaries time to settle. `2200 ms` is the empirical floor for AGH's `systems-*-routes-*` stories.
4. `Runtime.evaluate({ expression: "document.fonts.ready.then(()=>1)", awaitPromise: true, timeout: 5000 })` — block until Inter and JetBrains Mono are decoded and ready to paint. Without this step, captures intermittently render in fallback system fonts.
5. `Page.captureScreenshot({ format: "png" })` — capture; data comes back base64.
6. `writeFileSync(<out>/<name>.png, Buffer.from(data, "base64"))` — write the PNG.

## Capture sizing recommendations

| Surface class | Width × Height | Reason |
| --- | --- | --- |
| Full route (list / kanban / dashboard / detail) | 1440 × 900 | Matches a trusted prior baseline for the surface. |
| Wide route (validate column-snap breakpoints) | 1680 × 1050 | Catches lane-tab wrap and detail meta-line overflow. |
| Primitive (button, pill, dialog) | 1100 × 700 | Storybook's primitive stories render a tight preview; bigger viewports waste pixels. |
| Sidebar (collapsed) | 320 × 800 | Forces the collapse breakpoint. |

## Hard timeouts and failure modes

- If `Page.loadEventFired()` never resolves, the script hangs. Cap.mjs does NOT timeout — wrap external invocations with `timeout 90 bash -c '...'` when batching.
- If `document.fonts.ready` times out (`5000 ms`), `cap.mjs` emits `WARN <name> fonts.ready timeout` and proceeds with the snapshot. The PNG is usable but may show fallback fonts on slow networks.
- If `captureScreenshot` rejects, `cap.mjs` emits `FAIL <name> <message>` to stderr, continues the requested set, and exits 1 after cleanup.
- Small PNGs (under 20 KB) almost always mean Storybook rendered the "Couldn't find story" fallback. Re-check the story id against `list-stories.mjs`.

## Cleanup

The script always calls `chrome.kill()` in a `finally` block. If the helper is
killed before `finally`, use its printed debug port to identify the exact
headless Chrome listener, verify that PID's command line belongs to the failed
capture, and terminate only that PID/process group. Never use machine-wide
process-name cleanup in a concurrent workspace.
