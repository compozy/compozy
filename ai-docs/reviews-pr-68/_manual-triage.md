# Manual Triage For Extra Review Comments

This file tracks the additional comments provided outside the exported review threads.

## Implemented

- `pkg/compozy/runs/summary.go`
  - Wrapped `os.ReadDir` failures with directory context.
  - Removed the redundant `os.IsNotExist` branch.
  - Dropped the redundant `cleanWorkspaceRoot` call inside `runsDirForWorkspace`.
- `pkg/compozy/runs/examples_test.go`
  - `exampleEvent` now reflects the input `seq` in the JSON payload.
- `pkg/compozy/runs/tail.go`
  - Switched the live tail follower to `MustExist: true` to avoid an indefinite watcher goroutine.
- `pkg/compozy/runs/watch.go`
  - Logged dropped setup errors in `sendSetupError`.
  - Removed the dead `io.ErrUnexpectedEOF` branch in `isTransientRunLoadError`.
- `internal/core/agent/registry.go`
  - Introduced the `agent.RuntimeRegistry` interface and updated kernel dependencies to accept the interface instead of the concrete stateless registry.
- `internal/core/plan/prepare.go`
  - Preserved the caller context during journal cleanup by routing cleanup through `internal/core/preputil`.
- `internal/core/kernel/core_adapters.go`
  - Centralized migrate/sync/archive config bridges via typed `commands.*From*Config` helpers.
- `internal/core/kernel/handlers.go`
  - Replaced the duplicate journal cleanup path with the shared helper, which now logs close failures.
- `internal/core/run/execution.go`
  - `submitEvent` now errors when `jobExecutionContext` has no context instead of falling back to `context.Background()`.
  - `emitProviderCallCompleted` no longer fabricates a `200` status code when no real provider response code exists.
  - Added fallback event-bus attachment for UI-enabled executions prepared without a bus.
- `internal/core/run/exec_flow.go`
  - Removed the redundant branch from `execRunState.emit`.
  - Kept the `cfg.RunID` side effect but documented it explicitly because exec callers rely on it to discover persisted artifacts.
  - Made the no-op `publishExecFinish`/`publishExecRetry` placeholders explicit.
- `internal/core/run/execution_test.go`
  - Enabled `t.Parallel()` for the independent terminal-event subtests.
- `internal/core/run/ui_adapter_test.go`
  - Removed the flaky goroutine-count assertion.
  - Removed the ineffective `t.Helper()` call inside the cleanup closure.
  - Moved translation tests to a test-local helper after removing the one-shot production shim.
- `internal/core/run/ui_model.go`
  - Removed the one-shot `translateEvent` shim so production callers use a persistent translator instance.
- `pkg/compozy/events/kinds/session.go`
  - Collapsed the repetitive block decoders into a shared generic helper.

## Reviewed And Not Adopted

- `pkg/compozy/runs/examples_test.go`
  - The `t.TempDir()` suggestion is not applicable to `Example...` functions because they do not receive a `testing.TB`.
- `internal/core/agent/session.go`
  - The mutex-contention concern is already addressed in current code: `publish` releases `s.mu` before the potentially blocking wait and only uses a separate lock for the drop-warning bookkeeping.
- `internal/core/run/logging.go`
  - The comment about leaking `Done()` was stale; `HandleCompletion` already calls `markDone` on every terminal return path.
- `internal/core/run/logging.go`
  - The nil-context fallback in `newSessionUpdateHandler` was kept. That constructor returns a concrete handler, not an error, so removing the fallback would convert accidental nil contexts into panics during event submission rather than improving correctness.
- `internal/core/run/journal/journal.go`
  - The `context.Background()` used for live bus fan-out was kept intentionally. Journal fan-out is decoupled from individual submitter cancellation; reusing a canceled caller context would suppress live subscribers for events that were already durably written.
- `pkg/compozy/runs/tail.go`
  - The proposed removal of the `ctx.Err()` pre-checks was not adopted. With an already canceled context and an unbuffered consumer ready to receive, the `select` can still randomly choose the send case; the pre-check is what keeps `Tail()` from leaking canceled events into the output channel.
