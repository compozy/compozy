---
status: resolved
file: internal/core/run/execution.go
line: 184
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCF,comment:PRRC_kwDORy7nkc60y0gE
---

# Issue 009: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Non-blocking send may silently drop shutdown requests.**

The `select` with `default` means if the channel buffer (capacity 4) is full, the shutdown request is silently discarded. Consider logging when a request is dropped, or use a blocking send with timeout.


<details>
<summary>🔍 Suggested improvement</summary>

```diff
 func (c *executorController) requestShutdown(req uiQuitRequest) {
 	force := req == uiQuitRequestForce
 	select {
 	case c.shutdownRequests <- shutdownRequest{force: force, source: shutdownSourceUI}:
 	default:
+		// Channel full - shutdown already in progress
 	}
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func (c *executorController) requestShutdown(req uiQuitRequest) {
	force := req == uiQuitRequestForce
	select {
	case c.shutdownRequests <- shutdownRequest{force: force, source: shutdownSourceUI}:
	default:
		// Channel full - shutdown already in progress
	}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution.go` around lines 178 - 184, The non-blocking send
in executorController.requestShutdown can silently drop shutdownRequests when
the shutdownRequests channel is full; change the implementation in
requestShutdown to attempt to send but not silently discard: perform a send to
c.shutdownRequests with a bounded timeout (e.g., use time.After) and if the
timeout elapses log a warning including the request details (shutdownRequest,
uiQuitRequest and shutdownSourceUI) so dropped requests are visible; ensure the
timeout branch uses c.log or the existing logger to record the drop and do not
remove the original shutdownRequest type or channel.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:5915fb52-e9fa-449e-96f7-3838f12d9bae -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: The UI quit state machine emits at most one draining request followed by one force request, and `shutdownRequests` is buffered to 4. The non-blocking send is intentional coalescing so the UI thread never stalls if shutdown is already underway; dropping redundant requests does not change the executor state machine. Existing UI controller tests already cover the drain/force transition path.
