---
id: LP-error-route-fallback
area: LP
title: Follow an authored error route after retries are unavailable
persona: Lea
journey: J-recover-loop-node-failure
expected: A terminal node failure skips its success-only dependents, activates the forward fallback once, and finishes without leaking a fabricated output from the failed node.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run and event routes; web Loop run detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; `go test -tags=integration ./internal/daemon -run TestDaemonE2ELoopNodeLifecycleShouldRetryRouteAndEscalate -count=1`
last_report:
overlaps:
---

acceptance-walk: Publish a Loop with an exhausted retry policy, one authored error route, and a success-only dependent, then trigger the failure. Confirm the fallback runs once, the success-only dependent is skipped, no failed output enters the namespace, and refreshed Web, CLI, and HTTP reads agree on the final disposition.
