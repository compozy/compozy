---
id: ET-window-seam-resize-keyboard
area: ET
title: Resize split windows by dragging or keying the seam
persona: Bruno
journey: J-administer-window-manager
expected: Between two windows of a split the seam is a focusable separator named "Resize windows"; dragging it resizes both sides live and commits one `layout.resize`; a focused seam moves 2% per arrow key and 10% with Shift; double-clicking it balances the two sides it separates; a seam stops where a window would go below its app's pixel minimum; the new shares survive reload and appear in the tab's split `weights` from `compozy layout get`; `compozy layout resize --split <id> --boundary <n> --delta <d>` and `layout balance --tab|--split` do the same from the CLI.
entry_points: split seam (pointer, keyboard, double-click); compozy layout resize|balance; compozy__layout_resize|balance
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-layout-gestures; ET-web-tiled-window-shadow-flush; ET-window-arrange-presets
---

Added 2026-09-30 for the shell rail (VC-03 seam resize). Tiling is gutterless by default, so the seam is the only resize handle between panes.

Superseded walk (before tabs and splits; history only — walk the 2026-11-13 qa-impact note below): tile two windows side by side, hover the seam (2px accent), drag it to roughly 70/30, reload; Tab to the seam and press → five times (2% each) then Shift+→ once (10%); try to push a pane below 220px (it stops); undo and redo the layout; repeat on a horizontal seam in a stacked column; switch the theme and confirm the hairline and the accent state stay visible.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Island frame seams and the fixed 220px floor are gone; minimums come from each app. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Split two windows side by side. Drag the seam to about 70/30: both sides follow live, one layout change on release (⌘Z restores). Tab to the seam (focus ring visible), press → five times: about 10% moves; Shift+← once: 10% back. Double-click the seam: 50/50. Drag toward one side until it stops at that window's minimum. Reload: shares are kept. `compozy layout get -o json` shows the split's `weights`; `compozy layout resize --split <node-id> --boundary 0 --delta 0.1 --revision <r>` moves it, and `compozy layout balance --tab <tab-id> --revision <r>` evens it.

qa-impact: 2026-11-13 issue #1657. Held arrow keys no longer keep resizing after release: steps that wait behind a resize in flight on the same seam go out as one resize of their sum. Tab-wide Balance sizes (tab menu, Window › Arrange › Balance sizes, palette `layout.balance`) now evens only the splits that can be evened together without turning a side-by-side split into one member at a time; when all can, it is still one tab-wide `layout.balance`. A balance or move this client issues during a seam drag re-bases the drag instead of cancelling it. The seam's limit state appears with the frame that reaches the clamp. Walk: in the lab above, (1) on a slow link (devtools network throttling) hold → on a focused seam for two seconds and release: the seam stops within one step after release. (2) Split A | (B | C) with B and C uneven, narrow the window until evening A's split would squeeze B or C under its minimum, then Balance sizes from the tab menu: B | C evens, A's split keeps its shares, nothing collapses to one member at a time; this partial balance is one layout change per evened split, so ⌘Z undoes one split at a time, while a balance where every split fits stays a single ⌘Z. (3) Start dragging a seam and, with the pointer held, run Balance sizes from the palette shortcut: the drag continues from the balanced layout and its release lands where the preview shows.
