---
id: ET-window-manager-drop-swap
area: ET
title: Swap two windows by dropping with the swap modifier held
persona: Bruno
journey: J-administer-window-manager
expected: Holding the configured `swap_modifier` (default Shift) while dragging over any tiled window shows a whole-window "Swap windows" preview instead of split/insert/stack; releasing commits one `window.swap` that exchanges the two windows' structural places (tiled↔tiled, floating↔tiled, cross-desktop, and stack membership all exchange cleanly); releasing the modifier mid-drag restores structural drop previews on the next pointer move; `swap_modifier = "none"` disables the gesture; the same swap remains reachable via `compozy window swap` and `compozy__window_manager`.
entry_points: web desktop window drag; Settings window management swap modifier; compozy window swap; compozy__window_manager
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-01-window-tabs.md; internal/windowmanager/layout_commands_test.go
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-layout-gestures; MS-configure-window-manager; ET-window-manager-public-parity
---

story: As a builder, I can trade two windows' places in one gesture instead of rebuilding the layout around them.

scope: Include modifier press/release mid-drag, occupied targets under other candidates (z-order), group-move modifier held simultaneously, reduced motion, and revision conflicts during the drop.

qa-impact: 2026-07-24 added the modifier-gated swap drop surface over the existing `window.swap` semantic command, with the new `window_manager.swap_modifier` config key. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 tab drop targets now take precedence over structural snap and swap targets
and commit one group command. Reset as a gesture-arbitration canary.
