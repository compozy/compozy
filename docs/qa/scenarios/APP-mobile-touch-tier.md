---
id: APP-mobile-touch-tier
area: APP
title: Monitor from a phone-sized viewport on the touch tier
persona: Marina
journey: J-operate-desktop-shell
expected: At 390×844 the operator shell stays operable per the frozen T1–T8 artboard — 44px touch floor, keyboard-open keeps the composer above the keyboard, landscape flexes height — while desktop rendering is unchanged
entry_points: Web / (operator shell) at a 390×844 viewport; command palette via ⌘K
qa_status: pass
bug_ids: BUG-20260911-session-rail-docks-on-touch-tier
fix_status: fixed
retest_status: pass
fix_commits: 2adc10724
evidence: docs/qa/evidence/2026-09-11-mobile-surface-truth/m-10-portrait-home.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/m-11-palette-open.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/m-25-keyboard-open-548.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/m-27-landscape-390h.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/m-23-rail-open-390.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/d-10-desktop-1440-home.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/r-2-rail-open-390.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/r-3-rail-closed-390.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/r-4-landscape-rail-open.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/r-5-desktop-1440-rail-open.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/r-6-shadow-check-390.png
last_report: docs/qa/reports/2026-09-11-mobile-surface-truth.md
overlaps:
---

Walk the mobile monitoring journey against the normative artboard
`docs/design/opendesign/mobile-surface-truth/mobile-surface-truth-shell-390.html`
(frozen by the mobile-surface-truth design pass, branch `mobile-browser-pwa`):

- T1 touch floor (44px) on menubar controls, dock, palette rows/input, scroll pill.
- T3 compact win-layer reserves tab-bar height (stream content gains the dock band).
- T4 viewport contract `viewport-fit=cover, interactive-widget=resizes-content`;
  keyboard-open shrinks the layout viewport and the composer stays above it.
- T5 palette stays top-anchored at the touch tier; results well grows to
  min(52dvh, 440px).
- T6 session rail overlays the transcript at ≤760px instead of docking.
- T7 landscape keeps the same chrome and flexes height — verify the height-flex
  story, not 44px chrome in landscape (recorded residual).
- T8 loopback-only banner strip sits between menubar and win-layer.
- Confirm-or-bounce residuals: profile-switcher trigger 28px well; log bodies'
  own horizontal scroll wells.
- Desktop (>1024px) spot-check: rendering pixel-identical.

Composes with the mobile-surface-truth gating: on remote tiers the terminal
pane is absent (RT-gateway-operator-surface-truth owns that surface); this
scenario owns the viewport/touch ergonomics. Layout baseline UI-13
(`docs/qa/_seeds/final-qa/_children/12-web-ui.md`) covered the pre-ergonomics
layout and stays valid for unchanged surfaces.

QA walk 2026-09-11 (CH-mobile-shell-touch-tier, lab
`compozy-mobile-surface-truth-20260911-041346-834544`): T1 44px floor holds on
menubar controls, tab items, traffic lights, and palette rows; T5 palette
top-anchored at 76px with the results well at exactly min(52dvh, 440px)
(438.88px); T3/T7 compact work area reserves the 56px tab bar (no window
content under it; landscape stack ~289px with unchanged chrome); T4 viewport
contract present and the keyboard-open simulation (548px visual-viewport line)
keeps the composer and tab bar above the keyboard; desktop 1440×900 spot-check
matches the pre-change shape (floating dock, no tab bar). Profile-switcher
28px well confirmed as the recorded residual; log-body scroll wells and the
scroll pill were not reachable in the box (extensions log view not opened; no
scrollable stream without a live provider CLI). FAILS T6: at 390×844 the
sessions rail docks and crushes the composer to a 68px editor instead of
overlaying the transcript — BUG-20260911-session-rail-docks-on-touch-tier.
T8 loopback-only strip placement was not reachable in-product (no remote tier
in the lab); component-level coverage stays in UT-008. Keyboard-open was
emulated by shrinking the layout viewport (headless Chromium has no virtual
keyboard); the `interactive-widget=resizes-content` contract itself is
verified in the meta tag, and a real-device keyboard-open remains a
human-verification residual.

Repair re-walk 2026-09-11: T6 overlay verified at 390×844; landscape control
case unchanged.

Repair re-walk 2026-09-11 (BUG-20260911 fix, fresh lab
`compozy-mobile-surface-truth-repair-20260911-20260911-055549-379543`, daemon
`http://127.0.0.1:34657` serving a fresh `web/dist` build of the fix,
Playwright 1.62.1 device-emulated 390×844 touch): at 390×844 with the rail
open, the sidebar wrapper computes `display: block` / `position: absolute` /
`z-index: 30`, the rail overlays at x=0..264, the transcript keeps full width
(x=0, w=390), and the composer editor measures 332px wide (was 68) with the
send control intact; the wrapper's computed box-shadow carries the
`--shadow-overlay` layers (`rgba(0, 0, 0, 0.65) 0px 24px 48px -12px` plus the
1px light ring). Rail closed: wrapper w=0 with `box-shadow: none` (no sliver
ring), transcript full width. Landscape 844×390 control: rail still docks as a
layout column (transcript x=264 w=580, editor w=522) with no shadow — the
correct >760px rule, unchanged. Desktop 1440×900 spot-check: docked, no
shadow, behaviorally identical. Zero console errors. Root cause and
measurements in the bug's Fix/Verification sections; evidence
`r-1`–`r-6` under `docs/qa/evidence/2026-09-11-mobile-surface-truth/`.
