---
id: LP-cancel-restart-recovers
area: LP
title: Recover a canceled Loop coordinator during daemon restart
persona: Bruno
journey: J-recover-loop-node-failure
expected: Cancel remains idempotent when bound sessions are already absent, and a canceled coordinator task is repaired without preventing the daemon from becoming ready after every restart.
entry_points: `compozy loop cancel`; `compozy session remove`; `compozy task cancel`; `compozy daemon stop|start`; structured CLI reads
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-348-loop-cancel-restart-20260812-034715-623464-lab/qa-artifacts/qa/behavioral-evidence.md;/Users/pedronauck/dev/qa-labs/compozy-issue-348-loop-cancel-restart-20260812-034715-623464-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-12-issue-348-loop-cancel-restart.md
overlaps: LP-cancel-vs-kill
---

acceptance-walk: Start an active Loop, remove its bound sessions, request cooperative cancellation, and cancel the deterministic coordinator task before restarting the daemon twice. Confirm every start reaches readiness, cancellation reconciliation does not fail on the missing sessions, one replacement coordinator is reserved at a time, and fresh structured reads keep the Loop, task, and workspace in the correct workspace.
