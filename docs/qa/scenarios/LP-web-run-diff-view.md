---
id: LP-web-run-diff-view
area: LP
title: Compare two generations or two runs of one Loop
persona: Bruno
journey: J-replay-loop-history
expected: The diff view groups node rows by change kind using the CLI vocabulary, summarizes large values as size plus content hash with a link to full content, shows the divergence banner only when the two runs pin different definition versions, labels a still-executing side, renders an honest empty state when nothing differs, and never offers a cross-loop comparison.
entry_points: /loop-runs/$runId/diff deep link; inspect sheet Compare action
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can see exactly what changed between two attempts without reading two raw payloads side by side.

src: .compozy/tasks/graph-eng/task_08.md
