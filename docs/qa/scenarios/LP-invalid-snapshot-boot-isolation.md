---
id: LP-invalid-snapshot-boot-isolation
area: LP
title: Isolate one invalid Loop snapshot during boot
persona: Ada
journey: J-loop-terminal-recovery
expected: On daemon restart, a run whose persisted definition snapshot fails digest or strict decoding becomes terminal with a structured snapshot cause, while healthy runs restore and the daemon reaches readiness.
entry_points: compozy daemon; compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-terminal-loop-settlement; LP-crash-death-resume
---

The public product has no snapshot-corruption injector. A real-user walk may settle this as
`blocked-verify` with exact operator instructions; store-backed integration coverage remains the
automated contract and is not sufficient for a public-interface pass.
