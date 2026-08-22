---
id: LP-web-needs-you-gate-provenance
area: LP
title: Name the asking gate and round on needs-you cards
persona: Lea
journey: J-supervise-loop-steady-state
expected: A needs-you approval card names the asking gate in human words plus its round, while internal gate, request, node, and lifecycle enum values never appear in the default read.
entry_points: web /loop-runs/:id needs-you card; GET /api/workspaces/:workspace_id/loop-runs/:run_id/briefing
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/screenshots/task07-manual-needs-you-provenance.png
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-web-run-default-read-briefing
---

Walk an approval opened by a named gate in a later round. Read the card cold, confirm the gate label
and round identify the source, and scan the rendered default register for raw enum leakage.
