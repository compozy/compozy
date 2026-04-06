# Outside-of-diff from Comment 2

**File:** `internal/core/run/logging.go`
**Date:** 2026-04-06 10:19:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

- Disposition: VALID

- Rationale: O problema era real, mas o código atual já cobre o caso. `HandleCompletion()` chama `markDone(...)` em todos os caminhos de retorno relevantes, inclusive quando a emissão do evento terminal falha, então `Done()` não fica pendurado no encerramento da sessão.

## Details

<details>
> <summary>internal/core/run/logging.go (1)</summary><blockquote>
> 
> `199-240`: _⚠️ Potential issue_ | _🔴 Critical_
> 
> **Always close `done` on completion-path failures.**
> 
> If runtime-event submission fails in either terminal branch, `HandleCompletion` returns before `markDone`, so anything waiting on `Done()` can block forever and leak the session shutdown path.
> 
> 
> As per coding guidelines, "Every goroutine must have explicit ownership and shutdown via context.Context cancellation".
> 
> <details>
> <summary>🤖 Prompt for AI Agents</summary>
> 
> ```
> Verify each finding against the current code and only fix it if needed.
> 
> In `@internal/core/run/logging.go` around lines 199 - 240, The submit/emit error
> paths in the completion logic can return early without calling h.markDone,
> leaking anyone waiting on Done(); ensure h.markDone is always invoked before
> returning: in the failure branch, when emitErr != nil call h.markDone(err, true)
> before returning emitErr (so the original session error is signaled), and in the
> success/completed branch where submitRuntimeEvent returns an error call
> h.markDone(nil, false) (or appropriate non-error completion flag) before
> returning that error; adjust code around h.submitRuntimeEvent,
> writeRenderedLines, and the EventKindSessionCompleted/SessionFailed branches so
> every return from HandleCompletion invokes h.markDone.
> ```
> 
> </details>
> 
> </blockquote></details>
