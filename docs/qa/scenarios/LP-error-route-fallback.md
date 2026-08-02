---
id: LP-error-route-fallback
area: LP
title: Follow an authored error route after retries are unavailable
persona: Bruno
journey: J-recover-loop-node-failure
expected: A terminal node failure skips its success-only dependents, activates the forward fallback once, and finishes without leaking a fabricated output from the failed node.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run and event routes; web Loop run detail
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; `go test -tags=integration ./internal/daemon -run TestDaemonE2ELoopNodeLifecycleShouldRetryRouteAndEscalate -count=1`
last_report:
overlaps:
---

The isolated HTTP runtime walk confirms one fallback run, a skipped success-only path, and no
failed payload flowing into the namespace. Full verification remains blocked until task_07
publishes the lifecycle disposition and task_08 renders it; task_13 will independently read the
settled run after refresh across the remaining public surfaces.
