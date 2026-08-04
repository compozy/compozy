---
id: LP-sick-target-degrades-one-lane
area: LP
title: Keep healthy Loop lanes running when one target is sick
persona: Bruno
journey: J-recover-loop-node-failure
expected: Transport failures open only the affected family and target breaker, sick nodes fail fast with target_unavailable, healthy independent lanes keep running, and a successful half-open probe closes the breaker.
entry_points: `compozy loop nodes --state quarantined -o json`; `compozy loop status --run-id <run-id> -o json`; Loop run events over HTTP/SSE
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; public QA has no breaker-state or half-open probe injector
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: TA-loop-failure-breaker; LP-quarantine-diagnose-requeue
---

acceptance-walk: Run independent nodes against one failing target and one healthy target until the failing family-target breaker opens. Confirm only the sick lane fails fast with target_unavailable, the healthy lane keeps progressing, and a successful half-open probe closes the breaker in refreshed Web, CLI, and HTTP reads.
