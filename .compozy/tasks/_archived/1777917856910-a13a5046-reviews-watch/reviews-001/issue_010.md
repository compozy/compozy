---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/extension_bridge.go
line: 93
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22EK,comment:PRRC_kwDORy7nkc68_V6f
---

# Issue 010: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify non-test Go runtime code paths using context.Background().
# Expected: only startup/main paths should remain.
rg -n --type go 'context\.Background\(\)' -g '!**/*_test.go' -C2
```

Repository: compozy/compozy

Length of output: 1875

---

🏁 Script executed:

```shell
# Try to access and examine the file directly if available in working directory
find . -type f -name "extension_bridge.go" 2>/dev/null
```

Repository: compozy/compozy

Length of output: 97

---

🏁 Script executed:

```shell
# Read the extension_bridge.go file to examine the context issue
cat -n ./internal/daemon/extension_bridge.go | head -150
```

Repository: compozy/compozy

Length of output: 5289

---

🏁 Script executed:

```shell
# Search for all calls to normalizeRuntime to verify context propagation
rg -n 'normalizeRuntime' ./internal/daemon/extension_bridge.go -A2 -B2
```

Repository: compozy/compozy

Length of output: 355

---

🏁 Script executed:

```shell
# Check for other context.Background() usages in the same file
rg -n 'context\.Background\(\)' ./internal/daemon/extension_bridge.go
```

Repository: compozy/compozy

Length of output: 91

---

🏁 Script executed:

```shell
# Check which interfaces extensionBridge implements and what context requirements they have
rg -n 'DaemonHostBridge' --type go -A10 -B2 | head -50
```

Repository: compozy/compozy

Length of output: 3290

---

🏁 Script executed:

```shell
# Check if normalizeRuntime is called elsewhere or in tests
rg -n 'normalizeRuntime' --type go -B3 -A3
```

Repository: compozy/compozy

Length of output: 1658

---

🏁 Script executed:

```shell
# Check where StartRun is called from to understand if ctx could be nil
rg -n 'StartRun' --type go -B3 -A3 | head -80
```

Repository: compozy/compozy

Length of output: 5270

---

🏁 Script executed:

```shell
# Check if there are tests that pass nil context to StartRun
rg -n 'StartRun.*nil' --type go -B5 -A5
```

Repository: compozy/compozy

Length of output: 2925

---

🏁 Script executed:

```shell
# Check workspacecfg.Discover signature to see if it truly requires context propagation
rg -n 'func.*Discover' --type go -A3 -B1 | grep -A3 "workspacecfg\|Discover"
```

Repository: compozy/compozy

Length of output: 29612

---

🏁 Script executed:

```shell
# Find the workspace/config.go Discover function to confirm context checking
sed -n '58,75p' ./internal/core/workspace/config.go
```

Repository: compozy/compozy

Length of output: 566

---

🏁 Script executed:

```shell
# Check where extensionBridge.StartRun is called to see if ctx could legitimately be nil
rg -n 'bridge\.StartRun\|\.StartRun' ./internal/daemon --type go -B2 -A2 | head -50
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check the DaemonHostBridge interface definition
rg -n 'interface.*DaemonHostBridge\|type.*DaemonHostBridge' --type go -A10 -B2
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Search for DaemonHostBridge definition in extensions package
find . -name "*.go" -path "*/extension*" | xargs grep -l "DaemonHostBridge" 2>/dev/null | head -5
```

Repository: compozy/compozy

Length of output: 279

---

🏁 Script executed:

```shell
# Check the DaemonHostBridge interface definition
rg -n 'DaemonHostBridge' ./internal/core/extension/daemon_bridge.go -A15 -B2 | head -50
```

Repository: compozy/compozy

Length of output: 1059

---

🏁 Script executed:

```shell
# Find the actual interface definition
rg -n 'type DaemonHostBridge interface' --type go -A15
```

Repository: compozy/compozy

Length of output: 1187

---

🏁 Script executed:

```shell
# Verify what context.Cause does and confirm it cannot work with nil
sed -n '1,20p' ./internal/core/workspace/config_test.go | grep -A20 "TestDiscoverReturnsContextErrorWhenCanceled"
```

Repository: compozy/compozy

Length of output: 41

---



</details>

**Do not replace missing context with `context.Background()` in daemon runtime paths.**

The fallback at lines 91-93 bypasses context cancellation and deadline propagation. The function calls `resolveExtensionBridgeWorkspaceRoot(ctx)`, which invokes `workspacecfg.Discover(ctx)` — a function that explicitly checks `context.Cause(ctx)` for cancellation signals. By substituting `context.Background()`, you prevent proper lifecycle control of daemon-owned child runs.

Since all callers already pass a valid context (as required by the `DaemonHostBridge` interface), enforce this contract by returning an error instead:

<details>
<summary>Suggested change</summary>

```diff
-	if ctx == nil {
-		ctx = context.Background()
-	}
+	if ctx == nil {
+		return nil, fmt.Errorf("daemon: context is required")
+	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if ctx == nil {
		return nil, fmt.Errorf("daemon: context is required")
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/extension_bridge.go` around lines 91 - 93, The code currently
replaces a nil context with context.Background(), which breaks cancellation
propagation for daemon-run paths; instead, in the function that calls
resolveExtensionBridgeWorkspaceRoot(ctx) (in extension_bridge.go, part of the
DaemonHostBridge implementation), detect if ctx == nil and immediately return an
error (e.g., fmt.Errorf or wrapped error) indicating a missing context rather
than substituting context.Background(); keep
resolveExtensionBridgeWorkspaceRoot(ctx) and all downstream calls unchanged so
callers must pass a valid context.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1d9e0342-2d1f-4855-9a09-610af4474aea -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `normalizeRuntime` silently swaps a nil daemon context for `context.Background()`, which breaks cancellation/deadline propagation on extension-owned child-run paths.
- Fix plan: Reject nil context at the boundary and add a focused regression test in `internal/daemon/run_manager_test.go`, the minimal existing extension-bridge test surface outside the listed scope.
