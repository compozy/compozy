---
id: LP-web-roster-unknown-duration-cancellation
area: LP
title: Keep roster duration and cancellation provenance truthful
persona: Dora
journey: J-diagnose-loop-run-operator
expected: A roster row reads duration as unknown when timing was not retained, while strategy cancellation names its strategy cause and operator cancellation names its actor so neither is mistaken for the other.
entry_points: web /loop-runs/:id Inspect -> Nodes; GET /api/workspaces/:workspace_id/loop-runs/:run_id/nodes; compozy loop nodes --run <run-id> --all
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/screenshots/task07-manual-roster-cancellations.png
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-web-run-operator-register; LP-web-strategy-progress
---

Walk one row whose timing fields were not retained, one lane canceled by fan-out strategy, and one
node canceled by an operator. Compare the rendered roster with the structured roster read.
