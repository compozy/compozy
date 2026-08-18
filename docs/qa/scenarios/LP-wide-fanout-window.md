---
id: LP-wide-fanout-window
area: LP
title: Run a wide fan-out through a bounded active window
persona: Bruno
journey: J-complete-partial-loop
expected: A 500-lane fan-out with max_parallel 8 completes without creating more than eight active lanes, reports truthful pending and settled counts, and never executes a lane twice across restart.
entry_points: compozy loop validate|run|status; GET /loop-runs/:id over HTTP and UDS; compozy__loop_status; Loop SSE; config.toml loops.defaults.<kind>.fan_out_width; compozy config get|set|unset|show; /docs/loops/guardrails; /docs/loops/dsl-reference
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
