---
name: demo-video
description: Agent-recorded product demo videos — choreograph a real browser flow with a cursor overlay and camera zooms, encode from raw frames, and package for PR evidence or social posts. Project specifics come from .demo-video.yaml.
disable-model-invocation: true
argument-hint: "[flow to demo] [--target pr|social|raw] [--pr <n>]"
---

# Demo Video

Record a product demo the way a human presenter would — cursor gliding, clicking, typing through a real running app — by choreographing Playwright against it, capturing raw frames, and encoding with an animated camera (zoom in on moments, pull back for reveals). Outputs serve two destinations: **PR evidence** (this behavior works — often a QA scenario walked on camera) and **social/marketing** cuts.

`<skill-dir>` is the directory containing this SKILL.md. The pipeline is: flow file (per-demo choreography, agent-authored) → `recorder.mjs` (frames + camera track) → `encode.mjs` (video) → package. Frames are cached: reframing, trimming, and re-encoding never re-record.

## Hard rules

- **Surface scale is 1.5 or 2** over the viewport (e.g. 1920×1080 → 2880×1620). Fractional factors deliver zero frames; CSS zoom breaks hit-testing.
- **Frames over `recordVideo`:** capture goes through the recorder's `page.screencast` frame path — Playwright's built-in video writer has a hardcoded 1 Mbps ceiling.
- **The flow file lives in the project's gitignored workdir** (config `workdir`), under a package that resolves `@playwright/test` — the lib deliberately has no dependencies of its own.
- **State prep precedes capture.** Drive the app/API into the exact opening state (config `state` notes), verify with a screenshot, then `startCapture()`.
- **Self-eval before showing anyone:** walk the verify pass in `references/choreography.md`; present only output that passed it.
- **Posting to a PR or uploading anywhere happens only when the invocation named that destination**; otherwise artifacts stay local and the final message lists their paths.

## Procedure

**Step 1: Configure.** Read `.demo-video.yaml` at the repo root. When it is missing, copy `<skill-dir>/assets/demo-video.template.yaml` there, fill it from repo evidence (dev-server docs, e2e configs, package scripts), and flag any value that is a guess.
*Done when:* app URL, capture sizes, workdir, locator sources, and state notes are known.

**Step 2: Preflight.** Verify the app answers on the config URL (else start it via config `start`; if that needs a terminal the agent doesn't own, print a "🚨 YOUR ATTENTION IS REQUIRED 🚨" banner with the command). Verify `ffmpeg` is on PATH and `@playwright/test` + its chromium resolve from the workdir (`cd <workdir> && node -e "import('@playwright/test')"`).
*Done when:* health check passes and both tools resolve.

**Step 3: Scout.** Read the config `locators` sources for stable selectors covering the flow. Walk the flow once with plain Playwright screenshots (no capture) to confirm every selector and the state prep; on the first demo of a session, read `references/choreography.md` in full before continuing.
*Done when:* every step of the flow has a proven selector and the opening state is reproducible.

**Step 4: Choreograph.** Copy `<skill-dir>/assets/flow.template.mjs` to `<workdir>/<slug>/flow.mjs`; write `prepare()` from the config `state` notes and the choreography as beats + gestures per the craft reference.
*Done when:* the flow file runs the full journey in the head with no placeholder left.

**Step 5: Record.** `cd <workdir>/<slug> && node flow.mjs`. Check the printed stats: delivered fps ≥20 and every beat labeled. On zero frames, low fps, or capture stalls, read `references/recording-findings.md` before changing any capture parameter — the failure is likely one already measured there.
*Done when:* frames/, camera.json, and healthy stats exist.

**Step 6: Encode + self-eval.** `node <skill-dir>/scripts/encode.mjs --dir <workdir>/<slug>` (env: `OUT_W OUT_H FPS CRF START END TARGET_MB`; trim settle head/dead tail with `START`/`END`). Run the self-eval from `references/choreography.md` — ffprobe plus actually looking at extracted frames — and fix by re-encoding (reframe/trim) or re-recording (choreography flaw).
*Done when:* the self-eval passes.

**Step 7: Package.** Read `references/publishing.md` for the destination named by the invocation — PR (budgeted mp4 + attachment upload with drag-and-drop fallback, QA-evidence copy when configured), social (X-spec encode), or raw (keep the master). Report every artifact path, size, and — when posted — the comment URL.
*Done when:* the destination's checklist in the reference is satisfied.

## Error handling

- App down mid-run → restart via config `start`, re-run `prepare()`, re-record; frames from a broken run are not salvageable.
- Zero frames or fps <20 → `references/recording-findings.md` (fractional scale factor, surface too big, vsync flags) before touching parameters.
- A selector fails at record time that passed the scout → the state prep is nondeterministic; fix `prepare()`, not the choreography.
- Encode over `TARGET_MB` after the CRF ladder → shorten the cut (`START`/`END`) or lower `OUT_W`; the script says which it tried.
- `attach-github.sh` non-zero → the undocumented endpoint changed; use the manual drag-and-drop fallback in `references/publishing.md`.
- Missing `.demo-video.yaml` in a repo that never ran the skill → Step 1 bootstraps it; flag guessed values instead of silently proceeding.

## Bundled files

- `scripts/recorder.mjs` — recording library imported by flow files (mutating: drives the browser, writes frames + camera track). Zero bare imports; the flow injects the project's `chromium`.
- `scripts/encode.mjs` — camera-crop renderer + H.264 encoder (mutating; `--dir` required). `TARGET_MB` re-encodes at rising CRF until the file fits.
- `scripts/gif.mjs` — mp4 → palette-optimized GIF for inline image embeds (mutating).
- `scripts/attach-github.sh` — GitHub attachment upload via the undocumented drag-and-drop endpoint (mutating; prints comment markdown; non-zero exit → manual fallback).
- `assets/flow.template.mjs`, `assets/demo-video.template.yaml` — per-demo and per-project starting points.
- `references/recording-findings.md` — measured capture facts and the abandoned deterministic-capture spike; load before touching capture parameters or diagnosing capture failures.
- `references/choreography.md` — gesture/camera craft and the self-eval pass; load before authoring a flow.
- `references/publishing.md` — PR/social/QA-evidence packaging recipes; load at Step 7.
