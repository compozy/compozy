---
status: resolved
file: internal/core/agent/session.go
line: 78
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHBx,comment:PRRC_kwDORy7nkc60y0fs
---

# Issue 005: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Silent update dropping may mask backpressure issues.**

When the updates channel is full, `publish` silently discards the update. While this prevents blocking, it could lead to lost data without any indication. Consider logging when updates are dropped or using a metrics counter for observability.


<details>
<summary>🔧 Suggested observability improvement</summary>

```diff
 func (s *sessionImpl) publish(update model.SessionUpdate) {
 	s.mu.Lock()
 	defer s.mu.Unlock()
 	if s.finished {
 		return
 	}
 	if update.Status == "" {
 		update.Status = model.StatusRunning
 	}
 	s.updatesSeen++
 	select {
 	case s.updates <- update:
 	default:
+		// Consider adding observability here:
+		// slog.Debug("session update dropped due to backpressure", "session_id", s.id)
 	}
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/session.go` around lines 64 - 78, The publish method on
sessionImpl silently drops updates when s.updates is full; modify
sessionImpl.publish to record when an update is dropped instead of discarding
silently by detecting the default case of the select and emitting an observable
signal—e.g., increment a dropped counter or call a logger—so replace the empty
default with a call to a metrics counter or processLogger (or s.logger) noting
the dropped update along with identifying fields (status/session id) while
keeping the non-blocking behavior and existing s.updatesSeen increment.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:e9a5cb25-70ed-4597-839e-cc9e1e67363a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: `sessionImpl.publish` is intentionally lossy under backpressure so the ACP reader side never blocks on a slow consumer. This low-level transport type has no logger or metrics dependency, and adding one here would couple the agent transport to runtime observability concerns without fixing a demonstrated correctness bug.
