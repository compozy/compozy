---
id: TA-merge-gate
area: TA
title: Merge only through the merge-gate gate
persona: Bruno
journey: J-deliver-through-agents
expected: In a sandbox repository, merge-gate refuses moved heads and incomplete handoffs, checks live queue mode and branch rules, confirms actual landing before root sync, and serializes releases only on a direct operator message. Absent mode preserves direct merging; true uses the queue with both aggregates required; false requires backend heavy checks without a queue.
entry_points: compozy session prompt <merge-gate session> 'ready: #<pr> <sha>'
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

1. A valid handoff uses the mode read from the live variable and effective branch rules. An
   absent variable preserves direct merging; explicit `false` requires successful Verify and
   E2E aggregates without a queue. `true` requires those aggregates and the queue rule.
2. A head that moved after handoff is refused, and the sender is told to hand off again.
3. In absent mode, other pending/failing CI does not block the review/body criterion; report
   non-green checks after landing. In true/false mode, pending, failed, cancelled or missing
   aggregates refuse the handoff. A variable/rules mismatch always refuses without mutation.
4. A message without an exact head is refused.
5. A merge conflict goes to a child session, and the PR needs a fresh handoff after repair.
6. A blocking branch rule reports a refusal without `--admin` or changing settings. `--auto`
   is used only in verified queue mode, with the exact head. Queue entry is reported as pending;
   only `MERGED` permits root sync and the next merge. On dequeue or moved head, read the
   matching merge-group failure, clear only the gate's own invalid auto-merge request if still
   armed, and require a fresh handoff. Never automatically requeue.
7. A handed-off release request does nothing. A direct operator message runs a dry release that ends
   only at the published sandbox release. Queue mode waits for an empty queue, re-verifies
   the regenerated release head, and enqueues no other PR until publication completes.

W6 validation: workflow syntax, the actual merge-group selection shell and focused CI gate
self-tests are checked before delivery, with heavy execution in the PR's `ci:full` run.
The queue/mode walks above remain sandbox-only and unverified until maestro activates the
approved settings; the older report is not evidence for the new queue branches. The W6 PR
records the post-activation merge-group and failed-dependency checks required before normal
queue delivery begins. Never change the real repository's settings to run this scenario.
