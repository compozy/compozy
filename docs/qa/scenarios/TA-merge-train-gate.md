---
id: TA-merge-train-gate
area: TA
title: Merge only through the merge-train gate
persona: Bruno
journey: J-deliver-through-agents
expected: In a sandbox repository, merge-train refuses a moved head, a red check and a handoff without an exact head, merges a valid handoff bound to that head (or queues it when the sandbox `main` requires a merge queue), and runs a release only on a direct operator message.
entry_points: compozy session prompt <merge-train session> 'ready: #<pr> <sha>'
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-11-02-spec-cycle/QA.md
last_report: docs/qa/reports/2026-11-02-spec-cycle/QA.md
overlaps: TA-spec-conductor-delivery
---

Spec QA-005, covering US-009. This agent exists only in this repository, so run it in a sandbox
repository and never against the real remote.

Hand off from a same-workspace session and confirm each result names the failed condition:

1. A valid handoff merges at that head.
2. A head that moved after handoff is refused, and the sender is told to hand off again.
3. A failing required check is refused.
4. A message without an exact head is refused.
5. A merge conflict goes to a child session, and the PR re-enters the queue.
6. With a merge queue required on the sandbox `main`, a valid handoff reports `queued`, the next
   handoff is verified without waiting, and the queued PR later reports `merged`; a PR the queue
   removes reports a refusal naming the failing `merge_group` check.
7. A handed-off release request does nothing. A direct operator message runs a dry release that ends
   only at the published sandbox release.
