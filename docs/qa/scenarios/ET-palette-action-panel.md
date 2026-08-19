---
id: ET-palette-action-panel
area: ET
title: Run command and entity actions from the palette panel
persona: Bruno
journey: J-operate-command-palette
expected: Command-K on the selected palette row opens a filterable action panel anchored to that row. Command rows expose their runnable action plus Pin or Unpin, Set alias, and Set shortcut; unavailable commands expose only those meta-actions and the daemon reason. Entity rows expose only real domain actions, destructive actions are unmistakable, action chords work from anywhere inside the palette without repeating, and a row removed by refresh closes the panel without firing against the vanished target.
entry_points: Command-K; command palette command rows; command palette entity rows
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-web-command-palette-shortcuts
---

Flagged by command-palette task 04. Task 12 owns the first real-user walk, visual-contract
comparison, and verdict.
