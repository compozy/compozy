---
id: LP-web-fork-dialog
area: LP
title: Fork a linked run from a historical generation
persona: Bruno
journey: J-replay-loop-history
expected: The fork dialog defaults to the inspected generation, pre-fills the source run's declared inputs, submits to a new run and navigates to it, renders validation errors exactly as the run form does, and blocks submit with the daemon's deterministic reason when the source content is unavailable. Lineage then links both runs in each direction.
entry_points: /loop-runs/$runId inspect sheet Fork action; lineage block
qa_status: pass
bug_ids: BUG-20260818-loop-fork-inline-output
fix_status: fixed
retest_status: pass
fix_commits: e9c00c2
evidence: docs/qa/bugs/BUG-20260818-loop-fork-inline-output.md; docs/qa/reports/2026-08-18-graph-eng.md
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop operator, I can branch a what-if run from history without touching the original.

src: .compozy/tasks/graph-eng/task_08.md
