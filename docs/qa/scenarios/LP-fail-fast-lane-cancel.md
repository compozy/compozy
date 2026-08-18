---
id: LP-fail-fast-lane-cancel
area: LP
title: Cancel unfinished fan-out lanes after a definitive failure
persona: Bruno
journey: J-complete-partial-loop
expected: A fail_fast fan-out settles on the lowest-index definitive failure, keeps completed lane results, cancels every unfinished sibling through the bound session path, and records one bounded branch_pruned cause without treating strategy cancellation as failure.
entry_points: compozy loop validate|run|status; GET /loop-runs/:id; compozy__loop_status; Loop SSE
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

acceptance-walk: Publish a fail_fast fan-out with one completed lane, one definitively failed lane, and two live lanes. Confirm the live sessions receive cancellation, the completed lane remains unchanged, the failure path runs, and status history contains canceled_by_strategy plus one branch_pruned event.
