---
status: resolved
file: internal/core/run/executor/execution.go
line: 222
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzH,comment:PRRC_kwDORy7nkc644Msu
---

# Issue 008: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't gate shutdown cleanup on a canceled run context.**

`finalizeExecution()` also runs on interrupt/cancel paths. Waiting on observer hooks with the original `ctx` means this can fail immediately once the run is canceled, which skips the rest of finalization (`run.pre_shutdown`, terminal events, task meta refresh, sound notification). Use a bounded shutdown context that ignores caller cancellation here.

<details>
<summary>Suggested fix</summary>

```diff
-	if err := model.WaitForObserverHooks(ctx, internalCfg.RuntimeManager); err != nil {
+	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
+	defer cancel()
+	if err := model.WaitForObserverHooks(waitCtx, internalCfg.RuntimeManager); err != nil {
 		return fmt.Errorf("wait for pending observer hooks: %w", err)
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := model.WaitForObserverHooks(waitCtx, internalCfg.RuntimeManager); err != nil {
		return fmt.Errorf("wait for pending observer hooks: %w", err)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/executor/execution.go` around lines 220 - 222,
finalizeExecution() should not use the caller's cancelable ctx when waiting for
observer hooks because that causes shutdown steps to be skipped on cancellation;
change the call to model.WaitForObserverHooks to use a bounded shutdown context
that ignores the original ctx (e.g., create a new context via
context.WithTimeout(context.Background(), <reasonableDuration>) and defer
cancel()) and pass that to model.WaitForObserverHooks along with
internalCfg.RuntimeManager so the rest of finalization (run.pre_shutdown,
terminal events, task meta refresh, sound notification) still runs even if the
run ctx is canceled.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:88ec5306-112e-4d23-8001-452c7308ec4a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `finalizeExecution` waits for observer hooks using the caller's `ctx`, so a canceled run can abort shutdown before `run.pre_shutdown`, terminal event emission, metadata refresh, and sound notification.
  - Root cause: shutdown cleanup currently inherits cancellation semantics from the active run instead of using a bounded cleanup context.
  - Intended fix: wait for pending observer hooks with a short timeout on a context that ignores caller cancellation so finalization can complete deterministically.
  - Resolution: shutdown now waits for pending observer hooks on a bounded `context.WithoutCancel(...)` timeout, and a minimal executor regression test was added in `internal/core/run/executor/execution_test.go` to prove canceled run contexts still finalize cleanly.
