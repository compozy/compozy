---
id: LP-revise-repair-context
area: LP
title: Repair every rejecting gate from typed prior context
persona: Ada
journey: J-improve-loop-with-feedback
expected: Revise reruns the deterministic union of every route-causing gate's producers with previous verdicts and ordered route causes while carrying unrelated success, whereas an explicit in-body next_generation reruns the full body with origin gate_next_generation.
entry_points: compozy loop validate|run|status; HTTP/UDS Loop run/status routes; compozy__loop_status; Loop SSE replay; docs /docs/loops/reference-grammar and /docs/loops/guardrails; runtime E2E harness
qa_status: pass
bug_ids: BUG-20261005-loop-initial-gate-route-lost;BUG-20261005-loop-gate-route-bypasses-stall
fix_status: fixed
retest_status: pass
fix_commits: 20cbc5693d5e44d8157f994b9a71b11cf65b08f2
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/official-e2e-results.json;docs/qa/evidence/2026-10-02-untested/loops-gate-routing-replay-summary.json;docs/qa/evidence/2026-10-02-untested/loops-gate-routing-delivery-gate-v2.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: LP-010
---

Derived from the two in-body branches of `J-improve-loop-with-feedback`. Use multiple rejecting gates
to prove a stable producer union and route-cause order. Compare producer-scoped `revise` with the
fresh full-body `next_generation` route, including restart at the completion boundary and exact
parent/origin projection after claim fencing.

2026-10-05 adjacent regression: two public command-gate definitions reproduce lost
next_generation at initial admission and bypassed no-progress checks after an action
producer. CLI/HTTP/UDS and fresh Web evidence are recorded in the current report. This
reopens the affected established contract outside the original untested-scenario matrix;
prior multi-gate repair-context evidence is retained. Fix and real replay are pending.

2026-10-05 functional replay: initial next_generation, producer-present next_generation and
producer-present revise each stop as Stalled/no_progress in round 2 under window 2 and
cap 8. Typed generation origins, rejected verdicts and the absence of a third generation
agree across CLI/HTTP/UDS; production Web confirms the terminal state. The full Loop race
suite passes. See loops-gate-routing-replay-summary.json. Delivery gate and commit remain.

2026-10-05 delivery closure: both gate-routing findings are verified in 20cbc5693d5e44d8157f994b9a71b11cf65b08f2.
The final rebuilt daemon repeats root next_generation, producer next_generation and revise
as Stalled/no_progress at round 2 with no third generation. All required make gate lanes
pass and the committed tree equals the frozen tree. The earlier multi-gate repair-context evidence is retained; this adjacent regression is closed without adding a row to the original matrix.
