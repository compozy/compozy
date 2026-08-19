---
id: LP-runtime-provenance-observation
area: LP
title: Observe durable per-task runtime provenance
persona: Ada
journey: J-01
expected: Every completed task output reports the binder-applied provider, model, reasoning, speed, speed outcome, and source of each field; HTTP, UDS, CLI, `compozy__loop_status`, run SSE, and the read-only Web inspect view agree after daemon restart; no runtime edit control is shown; and another workspace cannot list, read, stream, or invoke the run.
entry_points: web loop run Inspect; compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id; compozy__loop_status; run SSE
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/cli-origin-dry-run.normalized.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/same-run-parity-v6.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/workspace-isolation-v6.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps: LP-044; LP-003
---

story: As an operator I can audit exactly which runtime executed each task and why that runtime won.

QA impact 2026-07-26: added by Compozy migration Task 06. Flag only; the next QA cycle owns cross-surface and accessibility validation.

src: .compozy/tasks/compozy-migration/task_06.md

QA impact 2026-08-19: Loop runtime selection now carries provider-neutral `normal|fast` speed and
the existing applied/unsupported/rejected outcome across every durable read surface. Reset to verify
cross-surface parity, restart continuity, and workspace isolation.
