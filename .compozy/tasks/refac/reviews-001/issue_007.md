---
status: resolved
file: internal/core/agent/registry_specs.go
line: 60
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZV9,comment:PRRC_kwDORy7nkc61XmQg
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, check the exact content of the registry_specs.go file to confirm the interface definition
cat -n internal/core/agent/registry_specs.go
```

Repository: compozy/compozy

Length of output: 13385

---

🏁 Script executed:

```shell
# Search for implementations of EnsureAvailable to see how it's used
rg "EnsureAvailable" -A 5 -B 2
```

Repository: compozy/compozy

Length of output: 6150

---

🏁 Script executed:

```shell
# Search for context.Background() usage in the codebase
rg "context.Background()" -B 3 -A 1
```

Repository: compozy/compozy

Length of output: 50374

---

🏁 Script executed:

```shell
# Look at the launcher verification code mentioned
cat -n internal/core/agent/registry_launch.go | head -100
```

Repository: compozy/compozy

Length of output: 2788

---

🏁 Script executed:

```shell
# Find resolveLaunchCommand implementation
rg "func.*resolveLaunchCommand" -A 15
```

Repository: compozy/compozy

Length of output: 1134

---

🏁 Script executed:

```shell
# Check if context.Background() is used in the availability check path
rg "resolveLaunchCommand|probeCommand" internal/core/agent/registry_launch.go -A 10 -B 2
```

Repository: compozy/compozy

Length of output: 1560

---



</details>

**Add `context.Context` as the first parameter to `RuntimeRegistry.EnsureAvailable`.**

This interface method calls external processes but lacks a context parameter, forcing the implementation to use `context.Background()` in `verifyLauncher`. This violates the guideline: "Pass `context.Context` as the first argument to all functions crossing runtime boundaries; avoid `context.Background()` outside `main` and focused tests." Without context plumbing, callers cannot cancel or time-box availability checks.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_specs.go` around lines 55 - 60, Update the
RuntimeRegistry interface so EnsureAvailable accepts context.Context as its
first parameter (change signature to EnsureAvailable(ctx context.Context, cfg
*model.RuntimeConfig) error), then update all concrete implementations of
RuntimeRegistry to match that signature and propagate the incoming ctx through
their external process calls; finally, replace any uses that call
EnsureAvailable (notably verifyLauncher) to pass the caller's ctx instead of
using context.Background(), ensuring context is threaded through to allow
cancellation/timeouts.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  Availability checks cross a runtime boundary via `exec.CommandContext`, but `EnsureAvailable` does not accept a caller context and `verifyLauncher` hardcodes `context.Background()`. That prevents cancellation and timeouts from propagating through the availability probe path. The fix is to thread `context.Context` through the registry interface, implementation, and call sites that perform runtime preparation.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
