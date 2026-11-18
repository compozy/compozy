---
id: ET-hidden-window-live-resources
area: ET
title: Suspend hidden window live resources
persona: Bruno
journey: J-operate-desktop-shell
expected: Task, Loop, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when off-desktop, in an inactive tab (alone or inside a split), or document-hidden; visible sibling windows stay current, and every window that becomes visible again reconciles current server state without a manual reload.
entry_points: Web OS shell; tab bar activation; desktop switch; browser background and foreground
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-11-frontend-performance/two-live-windows-visible.png;docs/qa/evidence/2026-08-11-frontend-performance/task-restored-cursor-reconnect.png;docs/qa/evidence/2026-08-11-frontend-performance/hidden-window-resources.har
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-visible-session-streaming
---

Task, Loop, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when off-desktop, in an inactive tab (alone or inside a split), or document-hidden; visible sibling windows stay current, and every window that becomes visible again reconciles current server state without a manual reload.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


QA impact 2026-10-17 (web performance): Subagent roster readers no longer subscribe while disabled and ignore unrelated query changes while enabled. Window visibility/resource ownership is unchanged. Re-walk hide/restore and verify the last stream observer still restores the authoritative list; the existing session-list integration suite covers the latter boundary.

Scope, measured results, and verification limits: [Web performance report](../reports/2026-10-17-web-performance.md). Historical scenario verdicts above are not new verification of this revision. Heavy local gates are deferred to CI by explicit user instruction.

QA impact 2026-10-17 (concurrent-session responsiveness): Repeat last-owner
hide/restore while both workspace-scoped and profile-scoped reads are mounted.
Fallback timers must re-arm for every observer, preserve unrelated sessions and
avoid fetching solely because the timer was recomputed. Query-client custom hash
defaults must retain the same behavior. The query-options/live-tail owning suites
and the [responsiveness report](../reports/2026-10-17-responsiveness.md) record the
current proof; this note does not upgrade the historical skipped browser verdict.

qa-impact: 2026-11-13 window-tabs-split S10. Minimize/restore and tab stacks are retired; a hidden window is now one on another desktop, in an inactive tab (alone or inside a split) or in a hidden document. Expected, the body paragraph and entry points rewritten. Walk: a Task and a dashboard window in a split in one tab, a Loop window in another tab; switch tabs and desktops and background the browser, and confirm only the hidden windows stop streams, polling and clocks and catch up when shown again. Reset for the current build.
