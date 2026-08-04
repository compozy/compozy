---
id: LP-transient-blip-heals
area: LP
title: Heal a transient node failure without starting a repair generation
persona: Lea
journey: J-recover-loop-node-failure
expected: One transport or attempt-timeout failure schedules one bounded retry, the next attempt succeeds, and public history shows no duplicate run or repair generation.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run and event routes; compozy__loop_status
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; `go test -tags=integration ./internal/daemon -run TestDaemonE2ELoopNodeLifecycleShouldRetryRouteAndEscalate -count=1`
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps:
---

acceptance-walk: Start a Loop whose first action attempt fails with a retryable transport error, restart the isolated daemon during durable backoff, and confirm the next attempt succeeds in the same generation. Compare refreshed Web, structured CLI, and HTTP event history to prove one run, two attempts, no repair generation, and no duplicate work.
