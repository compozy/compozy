---
status: resolved
file: internal/core/agent/client.go
line: 697
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Rl8p,comment:PRRC_kwDORy7nkc621sgo
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Check `process.Forced()` before treating `Wait() == nil` as a clean exit.**

`internal/core/subprocess/process.go` clears `waitErr` on forced termination, so `process.Wait()` returns `nil` after `Kill()` / escalated shutdown. With the current order, this path gets reported as `"ACP agent process exited before all sessions completed"` instead of `context.Canceled`.

<details>
<summary>🛠️ Suggested fix</summary>

```diff
 	err := process.Wait()
 
-	if err == nil {
-		c.failOpenSessions(errors.New("ACP agent process exited before all sessions completed"))
-		return
-	}
 	if process.Forced() {
 		c.failOpenSessions(context.Canceled)
 		return
 	}
+	if err == nil {
+		c.failOpenSessions(errors.New("ACP agent process exited before all sessions completed"))
+		return
+	}
 	c.failOpenSessions(err)
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/client.go` around lines 683 - 697, The current logic in
the goroutine using processRef() treats a nil return from process.Wait() as a
clean exit before checking process.Forced(), which misclassifies forced kills
because subprocess/process.go clears waitErr on forced termination; change the
order so you call process.Forced() first (after obtaining process :=
c.processRef() and ensuring it's non-nil) and call
c.failOpenSessions(context.Canceled) if Forced() is true, then handle the err ==
nil case to call c.failOpenSessions(errors.New("ACP agent process exited before
all sessions completed")), otherwise call c.failOpenSessions(err); keep the use
of process.Wait(), process.Forced(), and c.failOpenSessions exactly as named.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c1d29a9a-29d8-4bec-b3df-52129c8adbe5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `subprocess.Process.Wait()` clears `waitErr` after a forced termination, so `waitForProcess()` can see `err == nil` even when shutdown escalated via `Kill()` or forced teardown.
  - The current branch order checks `err == nil` before `process.Forced()`, which misclassifies forced exits as unexpected agent termination. I will reorder those checks and add a regression test that kills a real helper subprocess while a session is still open.
  - Resolved by checking `process.Forced()` first and adding `TestWaitForProcessTreatsForcedExitAsCancellation`.
