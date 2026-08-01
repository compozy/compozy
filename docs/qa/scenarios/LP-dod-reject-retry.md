---
id: LP-dod-reject-retry
area: LP
title: Retry rejected definition-of-done verification with context
persona: Ada
journey: J-improve-loop-with-feedback
expected: A rejected definition-of-done gate mapped to next_generation starts a fresh full-body generation with origin dod_retry and prior verdict context, remains bounded and atomic across interruption, and never terminalizes as a contract failure merely because more work is required.
entry_points: compozy loop validate|run|status; POST and UDS Loop run/status routes; compozy__loop_status; Loop SSE replay; docs /docs/loops/guardrails; runtime E2E harness
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/official-e2e-results.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps:
---

Derived from the definition-of-done branch of `J-improve-loop-with-feedback`. Interrupt between the
rejected verdict and next coordinator claim, then resume from durable state. The new generation must
carry the rejecting DoD verdict in `previous.verdicts.*`, rerun the full body, and respect the
iteration cap without a partial generation/event write.
