---
id: LP-revise-repair-context
area: LP
title: Repair every rejecting gate from typed prior context
persona: Ada
journey: J-improve-loop-with-feedback
expected: Revise reruns the deterministic union of every route-causing gate's producers with previous verdicts and ordered route causes while carrying unrelated success, whereas an explicit in-body next_generation reruns the full body with origin gate_next_generation.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run/status routes; compozy__loop_status; Loop SSE replay; docs /docs/loops/reference-grammar and /docs/loops/guardrails; runtime E2E harness
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/official-e2e-results.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps: LP-010
---

Derived from the two in-body branches of `J-improve-loop-with-feedback`. Use multiple rejecting gates
to prove a stable producer union and route-cause order. Compare producer-scoped `revise` with the
fresh full-body `next_generation` route, including restart at the completion boundary and exact
parent/origin projection after claim fencing.
