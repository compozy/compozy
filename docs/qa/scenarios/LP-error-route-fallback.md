---
id: LP-error-route-fallback
area: LP
title: Follow an authored error route after retries are unavailable
persona: Lea
journey: J-recover-loop-node-failure
expected: A terminal node failure skips its success-only dependents, activates the forward fallback once, and finishes without leaking a fabricated output from the failed node.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run and event routes; web Loop run detail
qa_status: pass
bug_ids: BUG-20260803-error-route-fallback-blocked
fix_status: fixed
retest_status: pass
fix_commits: Task 13 checkpoint
evidence: looprun-ce061b65a4695127; /Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/evidence/task13/13-route-fallback-done.png; internal/loop/coordinator_lifecycle_test.go
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps:
---

acceptance-walk: Publish a Loop with an exhausted retry policy, one authored error route, and a success-only dependent, then trigger the failure. Confirm the fallback runs once, the success-only dependent is skipped, no failed output enters the namespace, and refreshed Web, CLI, and HTTP reads agree on the final disposition.
