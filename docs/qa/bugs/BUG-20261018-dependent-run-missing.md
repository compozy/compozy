# BUG-20261018-dependent-run-missing: Completing preparation leaves the delivery task idle

- Status: verified
- Impact (user-side): Blocks-Completion
- Severity: High; Priority: P1
- Persona Affected: Bruno
- Journey Step: Complete a blocker and continue the dependency chain
- Scenarios: TA-027
- Origin: GitHub issue #798
- Report: docs/qa/reports/2026-10-18-issue-798-auto-enqueue.md

## Reproduction

Create workspace tasks A and B, with B opted into `--auto-enqueue-on-ready`. Add a `blocks` dependency from B to A. Run `task start A`, `task run start` and `task run complete` with A's returned run ID. Read B and `task run list B`.

Expected: B reaches ready with one automatically queued run. Before the fix, B reached ready but its run list stayed empty, requiring a manual start.

## Fix and verification

The auto-enqueue call was confined to leased completion publication. It now runs in shared completed-run publication after dependent reconciliation, preserving existing Loop-worker and enqueue guards. Fix commit: this change's implementation commit.

Regression test: existing `TestTaskManagerAutoEnqueueOnReadyEnqueuesDependentOnCompletionIntegration` with real SQLite, manual/leased completion, opt-out, concurrent blockers and completion replay. Manual cases failed before and passed after the fix. The real CLI/HTTP walk confirmed one queued run, persistence across daemon restart, and no duplicate on completion replay. Opt-out and failed-blocker canaries retained zero runs.
