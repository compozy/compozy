---
id: APP-terminal-desktop-fidelity
area: APP
title: Preserve terminal input and rendering in the packaged desktop
persona: Marina
journey: J-use-terminal-desktop
expected: The packaged desktop admits only its same-origin terminal socket and preserves native clipboard, accelerator resolution, zoom refit, CJK input, alternate-screen rendering, resize reflow, watcher equality, and primary-screen restoration.
entry_points: packaged desktop Terminal app; native Edit menu; desktop zoom controls
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: APP-desktop-native-edit-shortcuts; APP-desktop-page-zoom
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Open a terminal in the packaged desktop and verify same-origin socket access while cross-origin access is refused.
2. Exercise copy, paste, native accelerators, zoom, and CJK composition.
3. Run an alternate-screen program, resize it, and compare controller and watcher screens.
4. Exit the program and verify the primary screen is restored without stale alternate-screen content.
