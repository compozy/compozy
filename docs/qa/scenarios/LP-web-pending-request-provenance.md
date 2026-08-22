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
