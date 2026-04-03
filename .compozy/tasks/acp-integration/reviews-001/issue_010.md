---
status: resolved
file: internal/core/run/execution.go
line: 580
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCJ,comment:PRRC_kwDORy7nkc60y0gJ
---

# Issue 010: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Running job without semaphore slot when context is cancelled may cause resource contention.**

When `acquireWorkerSlot` returns `false` (context cancelled), the job still runs at line 574. This bypasses concurrency control and could lead to resource exhaustion during shutdown if many jobs start simultaneously.


<details>
<summary>🔍 Consider early return on cancellation</summary>

```diff
 func (j *jobExecutionContext) executeJob(jobCtx context.Context, index int, jb *job) {
 	defer func() {
 		j.wg.Done()
 		atomic.AddInt32(&j.completed, 1)
 	}()

 	if !j.acquireWorkerSlot(jobCtx) {
-		newJobRunner(index, jb, j).run(jobCtx)
+		// Context was cancelled while waiting for slot - skip execution
 		return
 	}
 	defer j.releaseWorkerSlot()

 	newJobRunner(index, jb, j).run(jobCtx)
 }
```
</details>

If running jobs during shutdown is intentional (e.g., to honor already-queued work), please add a comment clarifying this design choice.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func (j *jobExecutionContext) executeJob(jobCtx context.Context, index int, jb *job) {
	defer func() {
		j.wg.Done()
		atomic.AddInt32(&j.completed, 1)
	}()

	if !j.acquireWorkerSlot(jobCtx) {
		// Context was cancelled while waiting for slot - skip execution
		return
	}
	defer j.releaseWorkerSlot()

	newJobRunner(index, jb, j).run(jobCtx)
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution.go` around lines 567 - 580, The executeJob
function runs jobs even when acquireWorkerSlot returns false (likely due to
context cancellation), which bypasses semaphore concurrency control; change
executeJob to early-return when acquireWorkerSlot(jobCtx) returns false (so
newJobRunner(...).run is not called) or, if running jobs during shutdown is
intentional, add a clear comment in executeJob explaining that behavior and why
it bypasses acquireWorkerSlot; reference the acquireWorkerSlot,
releaseWorkerSlot, executeJob and newJobRunner(...).run symbols so the change is
made in this function and ensure releaseWorkerSlot is only deferred when a slot
was successfully acquired.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:5915fb52-e9fa-449e-96f7-3838f12d9bae -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: Although `executeJob` calls `run(jobCtx)` when `acquireWorkerSlot` returns false, `jobRunner.run` checks `ctx.Err()` before any ACP session setup and exits as canceled immediately. The existing regression test `TestJobExecutionContextLaunchWorkersReturnsPromptlyWithPendingACPJobs` already proves the queued second job never creates an ACP session after cancellation.
