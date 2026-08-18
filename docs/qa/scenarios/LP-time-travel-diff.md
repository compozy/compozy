---
id: LP-time-travel-diff
area: LP
title: Compare Loop generations and linked runs
persona: Bruno
journey: J-replay-loop-history
expected: Generation and same-Loop run diffs agree across CLI, HTTP, UDS, and native tools; carried cells, changed inputs, definition divergence, live as-of labels, and large-payload summaries are truthful.
entry_points: compozy loop diff; GET /loop-runs/:id/diff over HTTP and UDS; compozy__loop_diff; web /loop-runs/:runId/diff; /docs/loops/time-travel; /docs/loops/running
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can compare durable history without changing either run.

src: .compozy/tasks/graph-eng/task_07.md
