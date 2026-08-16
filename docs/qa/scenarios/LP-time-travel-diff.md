---
id: LP-time-travel-diff
area: LP
title: Compare Loop generations and linked runs
persona: Bruno
journey: J-inspect-loop-history
expected: Generation and same-Loop run diffs agree across CLI, HTTP, UDS, and native tools; carried cells, changed inputs, definition divergence, live as-of labels, and large-payload summaries are truthful.
entry_points: compozy loop diff; GET /loop-runs/:id/diff; compozy__loop_diff
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can compare durable history without changing either run.

src: .compozy/tasks/graph-eng/task_07.md
