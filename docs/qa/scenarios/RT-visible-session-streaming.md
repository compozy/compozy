---
id: RT-visible-session-streaming
area: RT
title: Keep every visible session stream live
persona: Théo
journey: J-13
expected: Two session windows visible side by side on the active desktop keep streaming when focus moves between them; minimizing, switching desktops, or hiding one behind an inactive stack suspends only that hidden window, which catches up when visible again.
entry_points: OS shell with two side-by-side session windows; web session live tail
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/visible-session-streaming-evidence.md
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: RT-023; RT-013
---

Added after the visible-window ownership rule changed from focus-only to actual OS-shell visibility.
