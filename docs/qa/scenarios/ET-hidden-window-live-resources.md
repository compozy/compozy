---
id: ET-hidden-window-live-resources
area: ET
title: Suspend hidden window live resources
persona: Bruno
journey: J-operate-desktop-shell
expected: Task, Loop, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when minimized, off-desktop, inside an inactive tab stack, or document-hidden; visible sibling windows stay current, and every restored window reconciles current server state without a manual reload.
entry_points: Web OS shell; window minimize and restore; desktop switch; tab-stack activation; browser background and foreground
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-11-frontend-performance/two-live-windows-visible.png;docs/qa/evidence/2026-08-11-frontend-performance/task-restored-cursor-reconnect.png;docs/qa/evidence/2026-08-11-frontend-performance/hidden-window-resources.har
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-visible-session-streaming
---

Task, Loop, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when minimized, off-desktop, inside an inactive tab stack, or document-hidden; visible sibling windows stay current, and every restored window reconciles current server state without a manual reload.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


QA impact 2026-10-17 (web performance): Subagent roster readers no longer subscribe while disabled and ignore unrelated query changes while enabled. Window visibility/resource ownership is unchanged. Re-walk hide/restore and verify the last stream observer still restores the authoritative list; the existing session-list integration suite covers the latter boundary.

Scope, measured results, and verification limits: [Web performance report](../reports/2026-10-17-web-performance.md). Historical scenario verdicts above are not new verification of this revision. Heavy local gates are deferred to CI by explicit user instruction.
