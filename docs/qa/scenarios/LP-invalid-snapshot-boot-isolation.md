---
id: LP-invalid-snapshot-boot-isolation
area: LP
title: Isolate one invalid Loop snapshot during boot
persona: Ada
journey: J-loop-terminal-recovery
expected: On daemon restart, a run whose persisted definition snapshot fails digest or strict decoding becomes terminal with a structured snapshot cause, while healthy runs restore and the daemon reaches readiness.
entry_points: compozy daemon; compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-26-loop-issues-fixes.md
last_report: docs/qa/reports/2026-08-26-loop-issues-fixes.md
overlaps: LP-terminal-loop-settlement; LP-crash-death-resume
---

The public product has no snapshot-corruption injector. A real-user walk may settle this as
`blocked-verify` with exact operator instructions; store-backed integration coverage remains the
automated contract and is not sufficient for a public-interface pass.

QA 2026-08-26: blocked at the public-interface boundary. A human verification requires an
operator-owned fixture that persists one invalid definition snapshot beside one healthy run, then
restarts the daemon and reads both runs through CLI or HTTP. No such public fixture exists today.
