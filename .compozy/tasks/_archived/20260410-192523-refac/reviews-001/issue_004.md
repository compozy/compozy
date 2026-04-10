---
status: resolved
file: internal/core/agent/registry_launch.go
line: 79
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZVy,comment:PRRC_kwDORy7nkc61XmQT
---

# Issue 004: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Use the effective model when probing availability.**

`EnsureAvailable` always resolves the launcher with `spec.DefaultModel`, so a user-supplied `cfg.Model` is never exercised during availability checks. For runtimes that bake the model into bootstrap args, bad model values will slip through until the real run starts.

<details>
<summary>💡 Suggested fix</summary>

```diff
 	if _, err := resolveLaunchCommand(
 		spec,
-		spec.DefaultModel,
+		resolveModel(spec, cfg.Model),
 		cfg.ReasoningEffort,
 		cfg.AddDirs,
 		cfg.AccessMode,
 		true,
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_launch.go` around lines 73 - 79, EnsureAvailable
currently calls resolveLaunchCommand with spec.DefaultModel, ignoring any
user-supplied cfg.Model; change it to use the effective model selection logic
(use cfg.Model when set, otherwise fall back to spec.DefaultModel) and pass that
effective model into resolveLaunchCommand so availability probing uses the same
model choice as runtime. Update the call in EnsureAvailable to compute
effectiveModel := cfg.Model; if effectiveModel == "" then effectiveModel =
spec.DefaultModel, then call resolveLaunchCommand(spec, effectiveModel,
cfg.ReasoningEffort, cfg.AddDirs, cfg.AccessMode, true).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  The review comment identifies a real limitation, but the suggested change does not fix it. `EnsureAvailable` only checks launcher presence and runs the launcher probe command; it never executes the full bootstrap command with model arguments. Replacing `spec.DefaultModel` with `cfg.Model` in the `resolveLaunchCommand` call would not validate the user-supplied model or change probe behavior, so this finding is not actionable in the current design.
  Resolution: analysis completed; no code change was warranted.
