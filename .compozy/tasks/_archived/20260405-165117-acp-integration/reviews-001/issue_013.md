---
status: resolved
file: internal/core/run/execution_acp_test.go
line: 577
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCE,comment:PRRC_kwDORy7nkc60y0gD
---

# Issue 013: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Potential race condition between publish and finish.**

The read lock is released before sending to the channel. Between `s.mu.RUnlock()` and `s.updates <- update`, another goroutine could call `finish()` which closes the channel, causing a panic on send to closed channel.


<details>
<summary>🐛 Suggested fix</summary>

```diff
 func (s *fakeACPSession) publish(update model.SessionUpdate) {
 	s.mu.RLock()
-	if s.finished {
-		s.mu.RUnlock()
-		return
-	}
-	s.mu.RUnlock()
-	s.updates <- update
+	finished := s.finished
+	s.mu.RUnlock()
+	if finished {
+		return
+	}
+	// Use select with default to avoid panic on closed channel
+	select {
+	case s.updates <- update:
+	default:
+	}
 }
```

Alternatively, use a write lock and hold it during the send, or use a sync.Once pattern for finishing.
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution_acp_test.go` around lines 569 - 577, The publish
method can race with finish because it releases s.mu before sending to
s.updates, allowing finish to close the channel; change publish
(fakeACPSession.publish) to hold the mutex while checking and performing the
send (e.g., acquire write lock or keep the read lock and promote to write) or
otherwise coordinate with finish (e.g., use a sync.Once/closed flag checked
under lock) so that the send to s.updates happens only while the session is
known-open; ensure the lock is released after the send to avoid the race and
avoid sending on a closed channel.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:e9a5cb25-70ed-4597-839e-cc9e1e67363a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: `fakeACPSession.publish` drops the lock before sending on `s.updates`, while `finish` closes that same channel under the mutex. That creates a real send-on-closed-channel race in the test helper.
- Fix plan: Coordinate `publish` and `finish` under the same mutex so a send cannot race a close.
- Resolution: `fakeACPSession.publish` now sends while holding the session mutex, eliminating the send-versus-close race with `finish`. `make verify` passed.
