---
id: LP-terminal-outcome-notification
area: LP
title: Deliver the effect for the terminal outcome that actually committed
persona: Lea
journey: J-recover-loop-node-failure
expected: Exactly the declared effect for the committed done, no-op, blocked, failed, exhausted, stalled, or canceled outcome is delivered once, and its result is observable without firing another lifecycle hook.
entry_points: web /loops/:name/editor; web /loop-runs/:id; compozy loop runs show <run-id> -o json; HTTP/UDS Loop events; SSE
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

acceptance-walk: Seed separate runs that commit done, no-op, blocked, failed, exhausted, stalled, and canceled, with distinct terminal effects. Confirm each run delivers only its committed terminal effect once and that refreshed Web, structured CLI, and HTTP event reads expose the same delivery result without recursive lifecycle effects.
