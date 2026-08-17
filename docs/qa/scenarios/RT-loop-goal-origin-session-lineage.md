---
id: RT-loop-goal-origin-session-lineage
area: RT
title: Loop Goal sessions retain informational origin lineage
persona: Ada
journey: J-15
expected: A Goal action reached from a session-started Loop creates or reuses one system session whose lineage points to the nearest session origin in the current or ancestor Loop Run. Structured session reads preserve the Goal when its origin is deleted, expose the recorded lineage without safe-spawn TTL, auto-stop, budget, permission narrowing, or active-child caps, and show a missing-origin Goal as self-rooted. Retries keep the first materialized lineage, spawned-session governance remains type-based, and transient ancestry store failures fail closed.
entry_points: session compozy__loop_run; Loop Goal action; session catalog HTTP/UDS/CLI; session ledger inspector
qa_status: pass
bug_ids: compozy/compozy#416
fix_status: fixed
retest_status: pass
fix_commits: ea021855; 49601716
evidence: docs/qa/evidence/2026-08-17-pr-420-review/structured-walk.md; docs/qa/evidence/2026-08-17-pr-420-review/teardown.json
last_report: docs/qa/reports/2026-08-17-pr-420-review.md
overlaps: ET-web-session-sidebar-threads; RT-session-parent-provenance; LP-run-loop-await-child-ordering
---

Added for issue #416. The isolated acceptance walk must start a child Loop from a real session,
observe the Goal system session under that origin across structured catalog reads, confirm the Goal
can use its declared tools, delete the origin without deleting or reaping the Goal, and confirm Loop
cleanup still stops the Goal. A separate missing-origin run must appear as a root. The Web projection
is settled independently by ET-web-session-sidebar-threads.

2026-08-16 targeted re-walk: the provider-backed origin called `compozy__loop_run`, the system Goal
carried the exact origin parent/root without spawn governance, and the origin was then removed. A
public child spawn from the still-live Goal succeeded with a five-minute TTL and rebased its new
root to the live Goal; HTTP and UDS parent projections matched byte for byte. The child and Goal
stopped cleanly. The missing-origin creation path remains covered by the owning integration suite.

2026-08-17 isolated completion walk: a real Goal session retained its origin across CLI/UDS and
HTTP, remained usable after that origin was removed, and safely spawned a child whose governance
root rebased to the live Goal. The structured projections matched and cleanup left no active
sessions. Verdict: pass.
