---
id: LP-runtime-provenance-observation
area: LP
title: Observe durable per-task runtime provenance
persona: Ada
journey: J-01
expected: Every completed task output reports the binder-applied provider, model, and reasoning plus the source of each field; HTTP, UDS, CLI, `compozy__loop_status`, run SSE, and the read-only Web inspect view agree after daemon restart; no runtime edit control is shown; and another workspace cannot list, read, stream, or invoke the run.
entry_points: web loop run Inspect; compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id; compozy__loop_status; run SSE
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-044; LP-003
---

story: As an operator I can audit exactly which runtime executed each task and why that runtime won.

QA impact 2026-07-26: added by Compozy migration Task 06. Flag only; the next QA cycle owns cross-surface and accessibility validation.

src: .compozy/tasks/compozy-migration/task_06.md
