---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: resolved
file: internal/daemon/task_multi_summary.go
line: 123
severity: medium
author: claude-code
provider_ref:
---

# Issue 006: Recovery summary can delay finalization by N times five seconds

## Review Comment

`collectTaskMultiRecoverySummary` reads child evidence serially, while each `readChildSummaryEvidence` call creates a fresh five-second timeout. Because summary collection runs before the parent terminal result is emitted, N unavailable or wedged child stores can delay an already-settled parent by `N * 5s`. This contradicts the stated best-effort invariant that a wedged child store must not hold finalization open and becomes severe for large multi-run queues.

Use one batch-level deadline or bounded parallel fan-out with owned cancellation so total summary latency has one fixed upper bound. Add a test with several blocked child stores and assert one aggregate deadline rather than one timeout per child.

## Triage

- Decision: `VALID`
- Notes: `collectTaskMultiRecoverySummary` reads children serially, while
  `readChildSummaryEvidence` creates a new detached five-second timeout for every
  child. `GlobalDB.GetRun` and `RunDB.ListEventsByKind` honor the supplied context,
  but restarting that context inside the loop makes the aggregate worst case
  `len(childRunIDs) * 5s` before the parent can emit its terminal result. The fix
  creates one detached batch deadline in the collector and passes that same
  context through every child evidence read. A focused regression test uses
  several context-blocked evidence readers and verifies they share one aggregate
  deadline rather than receiving sequential fresh deadlines.
- Verification note: the first full `make verify` run passed frontend checks,
  formatting, lint, and 5,298 Go tests, but an unrelated pre-existing timing race
  failed `internal/core/subprocess.TestShutdownEscalatesFromSIGTERMToSIGKILL`.
  Its 20ms SIGTERM grace can expire before the shell trap writes its marker; a
  focused `-race -count=10` reproduction passed nine times and failed once. The
  out-of-scope test remains unchanged; a subsequent full repository gate passed.
- Verification environment note: this review worktree's absolute path makes the
  default Playwright daemon socket exceed macOS Unix-socket path limits (`bind:
  invalid argument`). Foreground reproduction confirmed the path-length failure.
  The repository-supported `COMPOZY_HOME` override places only daemon runtime
  state under a short temporary path; `make frontend-e2e` then passed all seven
  Playwright tests without repository changes. Full verification scopes that
  override to Make's Bun command so Go home-resolution tests retain their normal
  environment.
- Final verification: the full repository `make verify` pipeline passed with
  frontend checks, zero Go lint issues, 5,298 Go tests, Go and extension builds,
  and all seven Playwright tests.
