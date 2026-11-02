---
id: LP-web-pending-request-provenance
area: LP
title: Name the asking step and round on pending requests
persona: Lea
journey: J-supervise-loop-steady-state
expected: A pending request card names the asking step in human words plus its round, so a supervisor can identify the source without opening Inspect or decoding a node id.
entry_points: web /loop-runs/:id needs-you card; GET /api/workspaces/:workspace_id/loop-runs/:run_id/briefing
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-14
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-web-run-default-read-briefing; LP-web-timeline-graph-rows
---

Open a run with a pending human request in a later round. Keep Inspect closed and confirm the card
alone identifies the asking step and round in plain language.

QA impact 2026-11-01 (responsiveness audit): request events continue to refresh
run detail, briefing/roster/story, workspace and All projects run/request lists,
and the workspace node inventory. Bursts share one in-flight read round; pending
reconciliation survives stream error or disposal. Recheck final request/provenance
state when a request is answered during an active read and the run view closes.
Focused results and pending runtime/current-head CI evidence are tracked in
`docs/qa/reports/2026-11-01-performance/loop-loops.md`.

QA impact 2026-11-02 (stream freshness review): request events can restart an older
active cached inventory read while preserving deferred invalidation for initial
or disabled reads. Disposal still retains accepted reconciliation. The focused
hook suite and lifecycle controls pass; runtime/CI verification remains with the
enclosing audit. Evidence: `docs/qa/reports/2026-11-01-performance/loop-loops.md`.

QA follow-up 2026-11-02 (independent queries): later request events must refresh
ready inventory/provenance reads while a different read is still pending. Accepted
work continues after disposal and disabled reads remain invalidated until enabled.
Focused controls pass; retain the enclosing runtime/CI walk. Evidence:
`docs/qa/reports/2026-11-01-performance/loop-loops.md`.
