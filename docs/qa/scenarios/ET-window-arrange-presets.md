---
id: ET-window-arrange-presets
area: ET
title: Arrange a desktop with Main and stack, Columns, Grid, or Balance sizes
persona: Bruno
journey: J-administer-window-manager
expected: Window › Arrange offers Main and stack, Columns, Grid, and Balance sizes for the focused window and the other visible windows on its desktop: Main and stack gives the focused window a 60% main column and splits the rest vertically beside it, Columns puts every visible window side by side, Grid tiles them in a grid, and Balance sizes evens the existing splits; each is one undoable layout change that keeps focus, minimized windows stay minimized, the same arrangements run from the command palette (layout.arrange.main-stack, layout.arrange.columns, layout.arrange.grid, layout.balance) and from `compozy layout arrange --arrangement main_stack|horizontal|grid` and `compozy__layout_arrange`; items are disabled with a reason, not hidden, when fewer windows make them meaningless.
entry_points: menubar Window › Arrange; command palette; compozy layout arrange; compozy__layout_arrange
qa_status: skipped
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
