---
id: ET-window-arrange-presets
area: ET
title: Arrange a desktop with Main and stack, Columns, Grid, or Balance sizes
persona: Bruno
journey: J-administer-window-manager
expected: Window › Arrange ▸ and a tab's Arrange ▸ offer Main and stack, Columns, Grid, and Balance sizes; an arrange gathers the focused window, the rest of its tab, and the front window of every other tab on the desktop into one split tab placed where the focused window's tab was (tabs left empty disappear): Main and stack gives the focused window a 60% main column and stacks the rest vertically beside it, Columns puts them side by side, Grid takes up to four windows, and Balance sizes evens every split in the tab; each is one undoable layout change, and `compozy layout arrange --arrangement horizontal|vertical|grid|main_stack` does the same for the listed windows.
entry_points: Window › Arrange; tab context menu › Arrange; palette `layout.arrange.*` and `layout.balance`; compozy layout arrange|balance; compozy__layout_arrange
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-menubar-menu-set; ET-window-seam-resize-keyboard; ET-window-manager-public-parity
---

Added 2026-09-30 for the shell rail (VC-04 arrange). `main_stack` is new in the `compozy__layout_arrange` / CLI enum and the palette gained `layout.arrange.main-stack` and `layout.arrange.columns` (unbound by default).

Walk: open four windows on one desktop, focus the second, run each Arrange item from the Window menu and check the geometry; repeat Main and stack from the palette and from the CLI (`--arrangement main_stack`, where the first listed window takes the main column); undo each; minimize one window and confirm Arrange leaves it minimized; with one window open confirm the items explain why they are disabled.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Arrange now builds one split tab instead of rearranging islands and decks; Columns no longer means "every visible window" but "the focused window, its tab, and the front window of each other tab". Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Open four apps as four tabs. Focus the first and choose Window › Arrange › Main and stack: one tab remains holding four windows, the focused one on the left at about 60%. Undo (⌘Z) restores four tabs. Choose Columns, then Grid (only four windows join; with a fifth tab open it stays a tab). Drag one seam, then Arrange › Balance sizes: all shares equal. Repeat from a tab's right-click Arrange ▸. CLI: `compozy layout arrange --desktop <id> --window <a> --window <b> --arrangement main_stack --revision <r>` produces the same tree in `compozy layout get`.
