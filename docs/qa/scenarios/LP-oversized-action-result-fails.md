---
id: LP-oversized-action-result-fails
area: LP
title: Reject an oversized Loop action result without leaking a lease
persona: Bruno
journey: J-complete-partial-loop
expected: A Loop action result above the 64 KiB contract fails the owning task and run with a bounded validation diagnostic, settles the active lease, creates no successful output, and leaves the Loop terminal or recoverable through the normal failure policy across CLI, HTTP, UDS, native tools, and fresh reads.
entry_points: Loop action execution; `compozy task runs`; Loop status over CLI/HTTP/UDS/native tools
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: blocked-verify
fix_commits: 75ce57f2
evidence: internal/loop/action_result_store_test.go;/Users/pedronauck/dev/qa-labs/compozy-pr-447-runtime-recovery-20260821-020432-748658-lab/qa-artifacts/qa/evidence/oversized-loop-probe.json
last_report: docs/qa/reports/2026-08-20-pr-447-runtime-recovery.md
overlaps: LP-run-agent-output-ownership; TA-action-run-liveness
---

Run an action that deterministically returns more than 64 KiB. Confirm no success is committed, the lease is no longer active, the task/run expose one bounded validation failure, and a fresh public read shows the authored Loop failure policy rather than a stranded running node.

QA 2026-08-20: a 70,000-byte builtin transform completed because that executor externalizes its result before the raw action-result boundary, so it was excluded as the wrong path. Human rerun: install a resource extension action that returns more than 64 KiB directly, execute it in a Loop, then confirm CLI and HTTP show a failed task/run, no active lease, no success output, and one bounded validation error.
