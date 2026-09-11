# BUG-20260911-session-rail-docks-on-touch-tier: On a phone-sized screen, opening the sessions list squeezes the composer to a sliver instead of overlaying it

- **Status:** fixed
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Marina
- **Journey Step:** J-operate-desktop-shell, session stream step (Touch Tier Tour, T6)
- **Scenarios:** APP-mobile-touch-tier
- **Found:** 2026-09-11 · **Report:** docs/qa/reports/2026-09-11-mobile-surface-truth.md

## Summary

Marina opens a session on her phone (390×844) and taps the sessions-list toggle to switch
sessions. Instead of the sessions rail sliding over the transcript as an overlay, it docks as a
solid 264px column: the transcript is compressed and the message composer is crushed into the
remaining ~110px strip (editor measured 68px wide; "Send a…" placeholder clipped, controls
stacked vertically). The screen is unusable while the rail is open; she has to close the rail to
type. The frozen mobile ergonomics map (T6 in
`docs/design/opendesign/mobile-surface-truth/mobile-surface-truth-shell-390.html`) promises the
opposite: at ≤760px the 264px rail overlays the transcript with an overlay shadow and the
transcript keeps its full width underneath.

## Reproduction

- **Charter:** CH-mobile-shell-touch-tier · **Tour:** Touch Tier Tour
- **Environment:** phone viewport 390×844 (device-emulated, touch), lab daemon `http://127.0.0.1:53001`, en-US

1. Open the operator shell, pick the registered workspace via the workspace picker (⌃⇧O palette entry).
2. Open a session (command palette → "New session" → Start session).
3. In the session window header, tap "Open sessions sidebar" (`session-sidebar-toggle`).

**Expected:** the 264px sessions rail overlays the transcript (shadow overlay), transcript keeps
full width; the composer stays intact and usable (T6).
**Actual:** the rail docks: it occupies x=0..264 as a layout column, the transcript/composer
compress into the remaining 126px, and the composer editor renders 68px wide.

## Evidence

- docs/qa/evidence/2026-09-11-mobile-surface-truth/m-23-rail-open-390.png (portrait 390: rail docked, composer crushed)
- docs/qa/evidence/2026-09-11-mobile-surface-truth/m-26-rail-portrait-evidence.png (second portrait capture)
- docs/qa/evidence/2026-09-11-mobile-surface-truth/m-28-landscape-rail-open.png (landscape 844×390: same docked shape — correct there per >760px desktop rule)
- Measured: rail column 0..264 at both viewports; portrait editor x=295 w=68; landscape editor x=295 w=522. No overlay shadow on the rail panel in either state.

## Fix

**Root cause:** the task_04 composition wrapper in
`web/src/systems/os/apps/session/session-window-content.tsx` relied on CSS
blockification — `display: contents` on the wrapper with media-scoped
`max-[760px]:absolute/inset-y-0/left-0/z-30/shadow-overlay` classes, assuming
`position: absolute` would force the contents wrapper to generate a box at the
touch tier. Chromium never blockifies `display: contents`: a minimal repro
(Playwright 1.62.1 Chromium) showed the computed display staying `contents`
with `position: absolute` set, no wrapper box, and the rail still participating
in the parent flex flow (transcript squeezed to 110px at a 390px viewport —
the bug's 126px shape). The built CSS did contain all the media-scoped rules
(verified in `web/dist/assets/index-*.css`), so the utilities shipped but were
inert: the rail docked at every width, and landscape only looked correct
because docking *is* the >760px desktop contract.

**Change (web/src/systems/os/apps/session/session-window-content.tsx only):**
the wrapper now opts back into a real display at the touch tier with
`max-[760px]:block`, so the absolutely positioned overlay box actually exists
at ≤760px (the media-scoped display rule sorts after the base `contents`
utility and wins the cascade inside the breakpoint). The overlay shadow rides
on the rail's open state (`sidebar.open && "max-[760px]:shadow-overlay"`), so a
closed rail cannot paint the shadow token's 1px ring down the transcript's
left edge. `SessionSidebar` and every other surface are untouched; desktop
(>760px) keeps the byte-identical `contents` wrapper. Added two regression
tests to the owning suite (`__tests__/session-window-content.test.tsx`)
pinning the touch-tier overlay composition (block/absolute/inset/z/shadow
while open) and the shadowless closed state.

## Verification

**Repair re-walk 2026-09-11** — fresh isolated lab
`compozy-mobile-surface-truth-repair-20260911-20260911-055549-379543`
(eng-qa-bootstrap, daemon `http://127.0.0.1:34657`, `COMPOZY_WEB_DIST_DIR`
serving a fresh `web/dist` build of the fix, lab `COMPOZY_HOME`/UDS per
manifest, teardown `clean: true`), Playwright 1.62.1 Chromium,
device-emulated 390×844 touch (DSF 3, en-US):

- **390×844, rail open (T6):** wrapper computes `display: block`,
  `position: absolute`, `z-index: 30`, `left/top: 0`; rail rect x=0 w=264
  (overlay, not a layout column); transcript rect x=0 **w=390 (full width
  kept)**; composer editor x=31 w=**332** (was 68) with the send button
  intact; computed `box-shadow` carries the `--shadow-overlay` layers
  (`rgba(0, 0, 0, 0.65) 0px 24px 48px -12px` plus the 1px light ring,
  browser-resolved `rgba(255, 255, 255, 0.043)`) over Tailwind's empty
  composition slots. Evidence:
  `docs/qa/evidence/2026-09-11-mobile-surface-truth/r-2-rail-open-390.png`,
  `r-6-shadow-check-390.png`.
- **390×844, rail closed:** wrapper box w=0, `box-shadow: none` (no sliver
  ring), transcript full width. Evidence: `r-3-rail-closed-390.png`.
- **Landscape 844×390 control:** rail docks x=0..264 as a layout column,
  transcript x=264 w=580, editor w=522 — identical to the pre-fix landscape
  shape, which is correct per the >760px desktop rule; no overlay shadow.
  Evidence: `r-4-landscape-rail-open.png`.
- **Desktop 1440×900 spot-check:** rail docked beside the transcript
  (aside x=174 w=264 in the windowed shell), no shadow — behaviorally
  identical to the pre-fix desktop rendering. Evidence:
  `r-5-desktop-1440-rail-open.png`.
- Zero browser console errors / page errors across all legs.
- Scoped validation from the repo root: `bunx turbo run typecheck --filter=./web`
  PASS (2/2 tasks); `bunx turbo run test --filter=./web` PASS — 776 test
  files / 7212 tests, 0 failures (baseline 7210 + 2 new regression tests);
  `bunx turbo run lint --filter=./web` PASS (0 warnings / 0 errors, format
  clean).
