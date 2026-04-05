# Outside-of-diff from Comment 2

**File:** `internal/core/run/execution.go`
**Date:** 2026-04-05 15:44:53 America/Sao_Paulo
**Status:** - [x] RESOLVED

- Disposition: INVALID

- Rationale: The cited code path is already fixed on this branch. `executeJobWithTimeout()` currently computes `emitHuman := cfg.humanOutputEnabled() && !useUI`, so the specific duplicate fallback described in this outside-of-diff comment is no longer present.

## Details

<details>
> <summary>internal/core/run/execution.go (1)</summary><blockquote>
> 
> `918-959`: _⚠️ Potential issue_ | _🟠 Major_
> 
> **Keep stderr fallback logs disabled while the UI is active.**
> 
> `emitHuman := cfg.humanOutputEnabled()` is too broad here. `handleNilExecution`, `handleSessionCancellation`, `handleSessionTimeout`, and `buildFailureResult` all write directly to `stderr`, so a TUI run now emits both UI events and raw fallback lines for the same failure/cancel path.
> 
> <details>
> <summary>Suggested fix</summary>
> 
> ```diff
> -	emitHuman := cfg.humanOutputEnabled()
> +	emitHuman := cfg.humanOutputEnabled() && !useUI
> ```
> </details>
> 
> <details>
> <summary>🤖 Prompt for AI Agents</summary>
> 
> ```
> Verify each finding against the current code and only fix it if needed.
> 
> In `@internal/core/run/execution.go` around lines 918 - 959, The code currently
> sets emitHuman := cfg.humanOutputEnabled() unconditionally which allows stderr
> fallback logs while the UI is active; change emitHuman to respect the UI by
> setting it to cfg.humanOutputEnabled() && !useUI (or equivalent logic) so that
> when useUI is true, emitHuman is false; update the local variable used in the
> call to handleSessionTimeout/recordFailureWithContext and the eventual
> executeSessionAndResolve invocation so all failure/cancel paths
> (handleNilExecution, handleSessionCancellation, handleSessionTimeout,
> buildFailureResult) will not emit raw stderr fallback lines while the TUI is
> active.
> ```
> 
> </details>
> 
> </blockquote></details>
