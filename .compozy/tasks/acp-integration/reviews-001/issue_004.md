---
status: resolved
file: internal/core/agent/registry.go
line: 572
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHBu,comment:PRRC_kwDORy7nkc60y0fp
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**`verifyLauncher` uses `context.Background()` instead of accepting a context parameter.**

Per coding guidelines, avoid `context.Background()` outside `main` and focused tests. The function should accept a `context.Context` parameter to allow proper cancellation and timeout propagation.


<details>
<summary>🛡️ Proposed fix</summary>

```diff
-func verifyLauncher(spec Spec, launcher Launcher) error {
+func verifyLauncher(ctx context.Context, spec Spec, launcher Launcher) error {
 	command := launcher.probeCommand()
 	if err := assertCommandExists(spec, command); err != nil {
 		return err
 	}

-	cmd := exec.CommandContext(context.Background(), command[0], command[1:]...)
+	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
```

Also update the call site in `resolveLaunchCommand`:
```diff
-func resolveLaunchCommand(
+func resolveLaunchCommand(
+	ctx context.Context,
 	spec Spec,
 	modelName string,
 ...
 		if verify {
-			if err := verifyLauncher(spec, launcher); err != nil {
+			if err := verifyLauncher(ctx, spec, launcher); err != nil {
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry.go` around lines 549 - 572, The verifyLauncher
function currently calls exec.CommandContext with context.Background(); change
verifyLauncher to accept a context.Context parameter (e.g., func
verifyLauncher(ctx context.Context, spec Spec, launcher Launcher) error),
replace context.Background() with the passed ctx when creating the command, and
update all callers (notably resolveLaunchCommand) to forward their context
through to verifyLauncher; ensure related tests and any other references are
updated to pass a context and preserve existing behavior (including error
construction using output and Cause).
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:5915fb52-e9fa-449e-96f7-3838f12d9bae -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: `verifyLauncher` is only reached through the current context-free preflight API (`EnsureAvailable` / `resolveLaunchCommand`). There is no reproduced failure here, and threading a `context.Context` through the real call chain would require widening public/internal APIs beyond this batch's scoped files. In the current implementation this is a short-lived availability probe, not a missing cancellation path in an otherwise context-aware runtime flow.
