---
status: resolved
file: internal/daemon/watchers.go
line: 261
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go8k,comment:PRRC_kwDORy7nkc651UNL
---

# Issue 022: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Preserve pending state when pre-sync reconcile fails**

On Line 258–260, a failed pre-sync reconcile returns after `state.pending` and `state.refreshWatches` were already cleared (Line 249–251). This drops queued artifact changes and can miss sync/emit after transient reconcile errors.


<details>
<summary>Suggested fix</summary>

```diff
 	changes := sortArtifactSyncEvents(state.pending)
-	state.pending = make(map[string]artifactSyncEvent)
 	refreshNeeded := state.refreshWatches
-	state.refreshWatches = false
+	state.pending = make(map[string]artifactSyncEvent)
+	state.refreshWatches = false
@@
 	if refreshNeeded {
 		if !w.reconcileWatchState(watcher, state, "daemon: refresh workflow watch list before sync") {
+			// Restore state so a later debounce/event can retry sync+emit.
+			for _, change := range changes {
+				state.pending[change.RelativePath] = change
+			}
+			state.refreshWatches = true
 			return
 		}
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/watchers.go` around lines 253 - 261, The pre-sync reconcile
clears state.pending and state.refreshWatches before calling
w.reconcileWatchState(watcher, state, ...) so a failed reconcile drops queued
changes; change the flow to preserve those flags until reconcileWatchState
succeeds — either move the clearing of state.pending and state.refreshWatches to
after a successful return from reconcileWatchState, or if you must clear
earlier, restore/re-queue them when reconcileWatchState returns false; update
the logic around reconcileWatchState, state.pending and state.refreshWatches to
ensure pending artifact changes are not lost on transient reconcile failures.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:97d23e0d-bd51-4027-84ca-f4d4931384fc -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `flushPendingChanges` clears `state.pending` and `state.refreshWatches` before the pre-sync `reconcileWatchState` call. If that reconcile fails, the function returns early and the queued changes are lost instead of being retried on the next debounce/event.
- Fix approach: preserve or restore the pending change map and refresh flag when the pre-sync reconcile fails, then add a regression test that forces reconcile failure and verifies the queued work remains in state without emitting changes.
- Resolution: `flushPendingChanges` now restores pending changes and the refresh flag on reconcile/sync failures before emit, and `TestWorkflowWatcherFlushPendingChangesPreservesStateWhenPreSyncReconcileFails` covers the regression.
- Verification: `go test ./internal/daemon` and `make verify`
