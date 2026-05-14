---
provider: coderabbit
pr: "151"
round: 1
round_created_at: 2026-05-14T00:24:33.853673Z
status: resolved
file: internal/core/agent/terminal.go
line: 129
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6B7N8J,comment:PRRC_kwDORy7nkc7BA5V8
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Don’t drop terminal tracking before the process is actually gone.**

If `waitFor(ctx)` returns on deadline/cancellation, this terminal has already been removed from `c.terminals`. After that, `Close()`/`Kill()` cannot retry cleanup, so the subprocess can be orphaned.




<details>
<summary>💡 Suggested fix</summary>

```diff
-	terminal, err := c.removeTerminal(params.SessionId, params.TerminalId)
+	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
 	if err != nil {
 		return acp.ReleaseTerminalResponse{}, err
 	}
 	terminal.kill()
 	if err := terminal.waitFor(ctx); err != nil {
 		return acp.ReleaseTerminalResponse{}, err
 	}
+	if _, err := c.removeTerminal(params.SessionId, params.TerminalId); err != nil {
+		return acp.ReleaseTerminalResponse{}, err
+	}
 	return acp.ReleaseTerminalResponse{}, nil
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	terminal.kill()
	if err := terminal.waitFor(ctx); err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	if _, err := c.removeTerminal(params.SessionId, params.TerminalId); err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	return acp.ReleaseTerminalResponse{}, nil
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/agent/terminal.go` around lines 123 - 129, The code currently
calls c.removeTerminal(...) before terminal.waitFor(ctx), which removes the
terminal from c.terminals and can orphan the subprocess on context deadline;
instead, look up the terminal without removing it (or fetch it from c.terminals
directly), call terminal.kill(), wait for it with terminal.waitFor(ctx), and
only after waitFor returns (successful cleanup or final error) call
c.removeTerminal(...) to remove tracking; update code paths that used
removeTerminal(...) to use the non-destructive lookup first, then call
removeTerminal by name/ID after waitFor completes.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `releaseTerminal` currently calls `removeTerminal(...)` before `terminal.waitFor(ctx)`.
  - If the caller context is already canceled or times out before the subprocess exit is observed, `waitFor` returns `ctx.Err()` after tracking has already been removed.
  - That makes the terminal unreachable for a follow-up `ReleaseTerminal`, `KillTerminalCommand`, or `WaitForTerminalExit`, which can orphan cleanup after a deadline-driven first attempt.
  - Fix approach: keep the terminal registered until `waitFor` completes successfully, then remove it from `c.terminals`; add client coverage for the retry-after-timeout path.
  - Resolution: `releaseTerminal` now looks up without removing, waits for completion, and only then drops tracking; `TestClientReleaseTerminalRetainsTrackingWhenWaitContextExpires` covers the retryable timeout/cancel path.
