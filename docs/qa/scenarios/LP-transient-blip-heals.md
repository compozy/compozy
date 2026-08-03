---
id: LP-transient-blip-heals
area: LP
title: Heal a transient node failure without starting a repair generation
persona: Ada
journey: J-recover-loop-node-failure
expected: One transport or attempt-timeout failure schedules one bounded retry, the next attempt succeeds, and public history shows no duplicate run or repair generation.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run and event routes; compozy__loop_status
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; `go test -tags=integration ./internal/daemon -run TestDaemonE2ELoopNodeLifecycleShouldRetryRouteAndEscalate -count=1`
last_report:
overlaps:
---

The isolated HTTP/SSE runtime walk passes the same-generation retry and classified prompt feedback.
Full verification remains blocked until task_07 exposes attempt and next-attempt fields on all
public read surfaces; task_13 will also exercise restart during backoff so the durable due-scan,
not an in-memory timer alone, proves recovery.
