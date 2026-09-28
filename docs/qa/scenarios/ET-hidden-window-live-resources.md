---
id: ET-hidden-window-live-resources
area: ET
title: Suspend hidden window live resources
persona: Bruno
journey: J-operate-desktop-shell
expected: Task, Loop, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when minimized, off-desktop, inside an inactive tab stack, or document-hidden; visible sibling windows stay current, and every restored window reconciles current server state without a manual reload.
entry_points: Web OS shell; window minimize and restore; desktop switch; tab-stack activation; browser background and foreground
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-11-frontend-performance/two-live-windows-visible.png;docs/qa/evidence/2026-08-11-frontend-performance/task-restored-cursor-reconnect.png;docs/qa/evidence/2026-08-11-frontend-performance/hidden-window-resources.har
last_report: docs/qa/reports/2026-08-11-frontend-performance.md
overlaps: RT-visible-session-streaming
---

Task, Loop, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when minimized, off-desktop, inside an inactive tab stack, or document-hidden; visible sibling windows stay current, and every restored window reconciles current server state without a manual reload.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.
