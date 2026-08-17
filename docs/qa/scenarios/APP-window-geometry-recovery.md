---
id: APP-window-geometry-recovery
area: APP
title: Restore usable window geometry across relaunch and display changes
persona: Dora
journey: J-desktop-attach-daily
expected: The main window restores its last usable normal bounds and maximized state after relaunch; saved bounds from a disconnected display recover fully inside a connected display without changing daemon ownership or app state.
entry_points: packaged desktop main-window move, resize, maximize, quit, relaunch, and disconnected-display recovery
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-desktop-page-zoom
---

Added 2026-08-16 for the Electron shell cutover. The macOS and Linux safety sessions own the
packaged-app walk. `desktop/e2e/_electron/__tests__/shell.spec.ts` automates maximized-state
persistence and off-screen recovery; the recorded walk owns exact normal-bounds restoration.
