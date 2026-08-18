---
id: LP-web-fork-dialog
area: LP
title: Fork a linked run from a historical generation
persona: Bruno
journey: J-replay-loop-history
expected: The fork dialog defaults to the inspected generation, pre-fills the source run's declared inputs, submits to a new run and navigates to it, renders validation errors exactly as the run form does, and blocks submit with the daemon's deterministic reason when the source content is unavailable. Lineage then links both runs in each direction.
entry_points: /loop-runs/$runId inspect sheet Fork action; lineage block
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can branch a what-if run from history without touching the original.

src: .compozy/tasks/graph-eng/task_08.md
