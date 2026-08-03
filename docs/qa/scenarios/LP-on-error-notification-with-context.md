---
id: LP-on-error-notification-with-context
area: LP
title: Deliver an on-error notification with the committed failure context
persona: Lea
journey: J-recover-loop-node-failure
expected: A declared on_error effect is delivered after the node failure commits, carries the exact node, attempt, disposition, failure, and run link, and records an isolated effect result without changing the node outcome.
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

acceptance-walk: Author one on-error emit effect, trigger the node failure, and confirm the effect appears only after the failure is committed. Compare the Web event story with structured CLI and HTTP event reads for the exact node, attempt, disposition, sanitized failure, run link, and isolated delivery result while the node outcome remains unchanged.
