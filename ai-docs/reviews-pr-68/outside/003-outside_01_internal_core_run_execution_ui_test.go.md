# Outside-of-diff from Comment 3

**File:** `internal/core/run/execution_ui_test.go`
**Date:** 2026-04-06 11:28:07 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** O fake de UI agora usa um canal bufferizado e `enqueue` não bloqueante, evitando hang do controller goroutine em testes que não drenam `f.ch`.

## Details

<details>
> <summary>internal/core/run/execution_ui_test.go (1)</summary><blockquote>
> 
> `323-333`: _⚠️ Potential issue_ | _🟡 Minor_
> 
> **Make the fake UI enqueue non-blocking or buffered.**
> 
> `newFakeUISession` creates an unbuffered `ch`, and nothing in this fake ever drains it. The first `enqueue` on this path will block the controller goroutine and hang the test.
> 
> <details>
> <summary>💡 Suggested change</summary>
> 
> ```diff
>  func newFakeUISession() *fakeUISession {
>  	return &fakeUISession{
> -		ch:          make(chan uiMsg),
> +		ch:          make(chan uiMsg, 8),
>  		waitCalled:  make(chan struct{}, 1),
>  		waitRelease: make(chan error, 1),
>  	}
>  }
>  
>  func (f *fakeUISession) enqueue(msg uiMsg) {
> -	f.ch <- msg
> +	select {
> +	case f.ch <- msg:
> +	default:
> +	}
>  }
> ```
> </details>
> 
> <details>
> <summary>🤖 Prompt for AI Agents</summary>
> 
> ```
> Verify each finding against the current code and only fix it if needed.
> 
> In `@internal/core/run/execution_ui_test.go` around lines 323 - 333, The fake UI's
> ch channel in newFakeUISession is unbuffered and enqueue blocks because nothing
> drains it; change newFakeUISession to create ch as a buffered channel (e.g.,
> make(chan uiMsg, N)) or modify fakeUISession.enqueue to perform a non-blocking
> send (use select with a default to drop or handle full queue) so enqueue never
> blocks the controller goroutine; update references to ch, newFakeUISession, and
> fakeUISession.enqueue accordingly.
> ```
> 
> </details>
> 
> </blockquote></details>
