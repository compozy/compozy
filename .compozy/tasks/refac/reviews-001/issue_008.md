---
status: resolved
file: internal/core/agent/registry_validate.go
line: 18
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWA,comment:PRRC_kwDORy7nkc61XmQk
---

# Issue 008: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Return an error for a nil runtime config instead of panicking.**

`ValidateRuntimeConfig(nil)` dereferences `cfg.Mode` immediately, so malformed caller input crashes the process instead of failing validation.


<details>
<summary>💡 Suggested fix</summary>

```diff
 func ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
+	if cfg == nil {
+		return errors.New("runtime config is nil")
+	}
 	if err := validateRuntimeMode(cfg.Mode); err != nil {
 		return err
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return errors.New("runtime config is nil")
	}
	if err := validateRuntimeMode(cfg.Mode); err != nil {
		return err
	}
	spec, err := lookupAgentSpec(cfg.IDE)
	if err != nil {
		return fmt.Errorf("invalid --ide value %q: must be %s", cfg.IDE, quotedSupportedIDEs())
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_validate.go` around lines 12 - 18,
ValidateRuntimeConfig currently dereferences cfg (cfg.Mode) and will panic when
called with nil; add an explicit nil check at the start of ValidateRuntimeConfig
to return a descriptive error (e.g., "runtime config is nil") instead of
panicking, then proceed to call validateRuntimeMode and lookupAgentSpec as
before; reference the ValidateRuntimeConfig function and ensure any callers
still get the same semantic errors from validateRuntimeMode/lookupAgentSpec when
cfg is non-nil.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:51576401-e056-421f-904c-06271e28cefe -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `ValidateRuntimeConfig` dereferences `cfg` immediately, so a nil runtime config panics instead of returning a validation error. This is a real defensive-validation bug. The fix is a nil guard at the top of the validator plus coverage for the nil case.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
