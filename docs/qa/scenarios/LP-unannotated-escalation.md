---
id: LP-unannotated-escalation
area: LP
title: Escalate an unhandled node failure into repair
persona: Lea
journey: J-recover-loop-node-failure
expected: A non-retryable failure with no route or absorption starts one next generation whose repair context preserves the classified cause and remediation hint.
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

acceptance-walk: Run a Loop whose non-retryable failure has no route and is not absorbed, then follow the automatic repair generation to completion. Confirm exactly one successor generation carries the classified cause and remediation hint, and compare fresh CLI and HTTP histories with the Web run story.
