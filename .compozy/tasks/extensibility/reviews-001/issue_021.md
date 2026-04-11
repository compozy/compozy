---
status: resolved
file: internal/core/extension/dispatcher.go
line: 206
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPD,comment:PRRC_kwDORy7nkc621Va4
---

# Issue 021: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Send the effective timeout to extensions, not just the manifest override.**

When `entry.hook.Timeout` is zero, Lines 189-194 still enforce `entry.extension.DefaultHookTimeout()`, but Line 205 serializes `timeout_ms` from `entry.hook.Timeout`, so the extension sees `0`. Any extension that honors `timeout_ms` will assume it has no deadline and can be canceled unexpectedly by the host.



<details>
<summary>Suggested fix</summary>

```diff
-			TimeoutMS: durationMilliseconds(entry.hook.Timeout),
+			TimeoutMS: durationMilliseconds(timeout),
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	timeout := entry.hook.Timeout
	if timeout <= 0 {
		timeout = entry.extension.DefaultHookTimeout()
	}
	if timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	request := executeHookRequest{
		InvocationID: d.nextInvocationID(),
		Hook: executeHookRequestHook{
			Name:      effectiveHookName(entry),
			Event:     hook,
			Mutable:   mutable,
			Required:  entry.hook.Required,
			Priority:  entry.hook.Priority,
			TimeoutMS: durationMilliseconds(timeout),
		},
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/dispatcher.go` around lines 188 - 206, The code
computes an effective timeout into the local variable timeout (falling back to
entry.extension.DefaultHookTimeout()) but still serializes
durationMilliseconds(entry.hook.Timeout) into the executeHookRequestHook,
causing extensions to receive the raw manifest value (often 0); update the
construction of executeHookRequestHook to use the computed timeout variable
(e.g. TimeoutMS: durationMilliseconds(timeout)) so the extension sees the actual
effective deadline used for callCtx; locate the code around
executeHookRequest/executeHookRequestHook, effectiveHookName,
durationMilliseconds, and d.nextInvocationID to make this change.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:388c7721-ae35-4ddb-a0e7-78fbd9aa7a58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: The dispatcher correctly computes an effective timeout in the local `timeout` variable, but it still serializes `entry.hook.Timeout` into the request payload. When the manifest timeout is omitted, the host enforces the extension default while the extension receives `timeout_ms=0`, which is a real contract mismatch. The fix is to serialize the computed effective timeout.
