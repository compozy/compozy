---
status: resolved
file: internal/core/extension/review_provider_bridge.go
line: 193
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iW,comment:PRRC_kwDORy7nkc624f7p
---

# Issue 012: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Error wrapping may obscure the actual failure reason.**

When the session isn't registered after startup, wrapping `shutdownErr` as the cause obscures the real issue. The primary failure is "session was not registered," but the error chain leads with the shutdown cleanup result.


<details>
<summary>🐛 Proposed fix to clarify error reporting</summary>

```diff
 	shutdownErr := manager.Shutdown(context.Background())
-	return nil, nil, fmt.Errorf(
-		"start review provider extension %q: session was not registered: %w",
-		name,
-		shutdownErr,
-	)
+	if shutdownErr != nil {
+		return nil, nil, fmt.Errorf(
+			"start review provider extension %q: session was not registered (shutdown error: %v)",
+			name,
+			shutdownErr,
+		)
+	}
+	return nil, nil, fmt.Errorf("start review provider extension %q: session was not registered", name)
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/review_provider_bridge.go` around lines 186 - 193,
The current return wraps shutdownErr as the cause which hides the real failure;
change the logic around manager.Shutdown so you first create the primary error
"start review provider extension %q: session was not registered" (using name)
and then, only if shutdownErr != nil, append the shutdown error as additional
context (but do not wrap it with %w as the primary cause). In other words,
return the session-not-registered error directly, and if manager.Shutdown
returned an error include that error text in the message (e.g., "...; shutdown
during cleanup: <err>") so the session failure remains the principal error while
still surfacing shutdownErr.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:7ee7cf70-ba4f-4569-ab3b-b2015b676f2c -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: the current `%w` wraps the cleanup shutdown error as the primary cause, which makes the error chain emphasize cleanup over the real failure: the session was never registered.
  - Fix plan: construct the session-registration failure as the primary error and append any cleanup shutdown failure as additional context instead of the wrapped cause.
  - Resolved: `internal/core/extension/review_provider_bridge.go` now preserves the session-registration failure as the wrapped cause and appends cleanup errors as context, with regression coverage in `internal/core/extension/review_provider_bridge_integration_test.go`; verified with `make verify`.
