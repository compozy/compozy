---
id: LP-unannotated-escalation
area: LP
title: Escalate an unhandled node failure into repair
persona: Ada
journey: J-recover-loop-node-failure
expected: A non-retryable failure with no route or absorption starts one next generation whose repair context preserves the classified cause and remediation hint.
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

The isolated HTTP runtime walk confirms generation-one failure, one repair generation, and the
classified cause in automatic repair feedback. Full verification remains blocked until task_07
makes disposition and attempt history public; task_13 will repeat the walk across those reads.
