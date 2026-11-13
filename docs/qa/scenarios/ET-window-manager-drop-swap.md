---
id: ET-window-manager-drop-swap
area: ET
title: Swap two windows by dropping with the swap modifier held
persona: Bruno
journey: J-administer-window-manager
expected: Holding the configured `swap_modifier` (default Shift) while dropping a dragged window header on another window of the same tab shows a whole-window Swap target instead of an edge target; releasing commits one `window.swap` that exchanges the two windows' places in the split; releasing the modifier mid-drag returns to edge targets; across tabs the modifier is ignored and no swap target is offered; `compozy window swap` and `compozy__window_swap` swap two windows of the same tab and reject windows in different tabs with `window_manager_invalid_command`.
entry_points: window header drag with Shift held; compozy window swap; compozy__window_swap; [window_manager] swap_modifier
qa_status: untested
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

qa-impact: 2026-11-13 window-tabs-split S10. Swap is limited to two windows of the same tab; floating and cross-desktop swaps no longer exist. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Build a tab with three windows split side by side (drag tabs onto the content). Drag the left window's header over the right window, hold Shift: the target reads Swap; release: the two windows exchange places and the middle one stays. Undo restores them. Drag again, press and release Shift mid-drag: edge targets return. Drag a header with Shift over a window of another tab (via the bar dwell): no Swap target. CLI: `compozy window swap --first <a> --second <b> --revision <r>` across two tabs returns `window_manager_invalid_command` and the revision does not advance.
