---
id: LP-wide-fanout-window
area: LP
title: Run a wide fan-out through a bounded active window
persona: Bruno
journey: J-author-wide-fanout
expected: A 500-lane fan-out with max_parallel 8 completes without creating more than eight active lanes, reports truthful pending and settled counts, and never executes a lane twice across restart.
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

acceptance-walk: Publish and run a 500-lane Loop with max_parallel 8. Observe progress while lanes advance, restart the daemon mid-window, and confirm the run finishes with every lane executed once, at most eight active lanes, and no fan-out ceiling error.
