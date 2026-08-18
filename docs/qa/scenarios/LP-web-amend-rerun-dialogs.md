---
id: LP-web-amend-rerun-dialogs
area: LP
title: Amend a parked node output and rerun from a settled node
persona: Bruno
journey: J-replay-loop-history
expected: Amend is absent on a node with no declared output shape and offered on a parked node that has one; its dialog shows the recorded original read-only beside a schema-validated editor and reconciles from refreshed truth. Rerun is absent while a node is parked or a generation is in flight; its dialog previews the rerun set and carried count before committing.
entry_points: /loop-runs/$runId node row actions; node control menu
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can correct a bad output or re-execute from a healthy node, and I am never offered a verb the daemon would refuse.

src: .compozy/tasks/graph-eng/task_08.md
