# Outside-of-diff from Comment 2

**File:** `internal/core/agent/session.go`
**Date:** 2026-04-06 10:19:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

- Disposition: VALID

- Rationale: O comentário procede, mas já estava corrigido no worktree atual. `publish()` não segura mais `s.mu` durante a espera de backpressure; ele contabiliza o publish ativo, libera o lock antes da espera bloqueante e `finish()` aguarda os publishes em voo antes de fechar o canal.

## Details

<details>
> <summary>internal/core/agent/session.go (1)</summary><blockquote>
> 
> `129-162`: _⚠️ Potential issue_ | _🟠 Major_
> 
> **Mutex held during blocking wait may cause contention.**
> 
> The `publish` method holds `s.mu` for up to `sessionPublishBackpressureTimeout` (5 seconds) while waiting in the second `select` block (lines 153-161). This could block other goroutines calling `Identity()`, `Err()`, `finish()`, or other mutex-protected methods, potentially causing cascading delays or deadlocks under high load.
> 
> Consider restructuring to release the mutex before the blocking wait, re-acquiring only to update state if needed.
> 
> 
> 
> <details>
> <summary>🔧 Suggested restructuring approach</summary>
> 
> ```diff
>  func (s *sessionImpl) publish(ctx context.Context, update model.SessionUpdate) {
>  	s.mu.Lock()
> -	defer s.mu.Unlock()
>  	if s.finished {
> +		s.mu.Unlock()
>  		return
>  	}
>  	if update.Status == "" {
>  		update.Status = model.StatusRunning
>  	}
>  	s.updatesSeen++
>  	s.lastUpdateWasFailedToolCall = update.Kind == model.UpdateKindToolCallUpdated &&
>  		update.ToolCallState == model.ToolCallStateFailed
>  	if s.suppressUpdates {
> +		s.mu.Unlock()
>  		return
>  	}
> +	s.mu.Unlock()
> +
>  	select {
>  	case s.updates <- update:
>  		return
>  	default:
>  	}
>  
>  	timer := time.NewTimer(sessionPublishBackpressureTimeout)
>  	defer timer.Stop()
>  
>  	select {
>  	case s.updates <- update:
>  		s.slowPublishes.Add(1)
>  	case <-timer.C:
>  		droppedTotal := s.droppedUpdates.Add(1)
> -		s.warnDroppedUpdate(update.Kind, droppedTotal)
> +		s.warnDroppedUpdateLocked(update.Kind, droppedTotal)
>  	case <-ctx.Done():
>  		return
>  	}
>  }
> +
> +func (s *sessionImpl) warnDroppedUpdateLocked(kind model.SessionUpdateKind, droppedTotal uint64) {
> +	s.mu.Lock()
> +	defer s.mu.Unlock()
> +	// ... existing rate-limiting logic
> +}
> ```
> </details>
> 
> <details>
> <summary>🤖 Prompt for AI Agents</summary>
> 
> ```
> Verify each finding against the current code and only fix it if needed.
> 
> In `@internal/core/agent/session.go` around lines 129 - 162, The publish method
> currently holds s.mu while doing a blocking send which can block other
> mutex-protected methods; change publish (sessionImpl.publish) to only hold s.mu
> for quick state checks/updates (set default Status, increment s.updatesSeen, set
> s.lastUpdateWasFailedToolCall, read s.suppressUpdates) and attempt the
> non-blocking send to s.updates while still locked; if that fails and you need to
> wait, release s.mu before creating the timer and performing the blocking select
> to send to s.updates or time out; after a blocking send completes (or times out)
> re-acquire s.mu only to update shared counters s.slowPublishes or
> s.droppedUpdates and call s.warnDroppedUpdate(update.Kind, droppedTotal) so that
> the long wait is not performed with s.mu held.
> ```
> 
> </details>
> 
> </blockquote></details>
