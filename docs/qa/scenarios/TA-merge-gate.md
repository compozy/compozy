---
id: TA-merge-gate
area: TA
title: Merge only through the merge-gate gate
persona: Bruno
journey: J-deliver-through-agents
expected: In a sandbox repository, merge-gate refuses a moved head and a handoff without an exact head, merges a reviewed handoff directly even with pending or red CI, reports non-green checks after root sync, and runs a release only on a direct operator message. There is no merge queue; main may go red briefly and is fixed right after.
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

1. A valid handoff merges at that head.
2. A head that moved after handoff is refused, and the sender is told to hand off again.
3. Pending or failing CI does not block a handoff that meets the CodeRabbit rounds or fallback
   criterion with zero unresolved threads; after merging, root sync runs and non-green checks
   are listed for immediate repair on main.
4. A message without an exact head is refused.
5. A merge conflict goes to a child session, and the PR needs a fresh handoff after repair.
6. A remaining branch rule that blocks a direct merge reports a refusal naming the rule,
   without retrying with `--admin` or `--auto` or changing repository settings.
7. A handed-off release request does nothing. A direct operator message runs a dry release that ends
   only at the published sandbox release.
