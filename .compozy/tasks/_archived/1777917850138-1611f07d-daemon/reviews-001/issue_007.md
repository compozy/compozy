---
status: resolved
file: internal/api/core/handlers.go
line: 885
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm5,comment:PRRC_kwDORy7nkc65HKYE
---

# Issue 007: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Don't swallow `stream.Close()` errors.**

This defer is the only cleanup path for the opened stream. If close fails, the handler currently hides it completely; log it with the run id at minimum.


<details>
<summary>Suggested fix</summary>

```diff
 	defer func() {
-		_ = stream.Close()
+		if err := stream.Close(); err != nil && h != nil && h.Logger != nil {
+			h.Logger.Warn("close run stream", "run_id", c.Param("run_id"), "error", err)
+		}
 	}()
```
</details>

As per coding guidelines, "NEVER ignore errors with `_` — every error must be handled or have a written justification".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	defer func() {
		if err := stream.Close(); err != nil && h != nil && h.Logger != nil {
			h.Logger.Warn("close run stream", "run_id", c.Param("run_id"), "error", err)
		}
	}()
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers.go` around lines 883 - 885, The defer currently
swallows the error from stream.Close(); change it to capture and log the error
with the run id instead of assigning to `_`. Replace the defer func that calls
`_ = stream.Close()` with code like: defer func() { if err := stream.Close();
err != nil { /* use the existing logger in scope (e.g., logger/processLogger) */
logger.Errorf("stream.Close failed for run %s: %v", runID, err) } }() so the
Close error for stream.Close() is not ignored and includes the run id for
diagnostics.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c1d7e4c5-68cf-4aef-a285-1de9756bb650 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `StreamRun` swallows the only cleanup error from `stream.Close()`, which violates the repo policy against ignored errors and hides diagnostics when transport cleanup fails.
- Fix plan: log close failures with the run id via the handler logger and add a focused regression test for the logged warning path.
- Resolution: Implemented and verified with `make verify`.
