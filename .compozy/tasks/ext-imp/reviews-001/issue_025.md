---
status: resolved
file: internal/core/workspace/config_validate.go
line: 128
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0ik,comment:PRRC_kwDORy7nkc624f75
---

# Issue 025: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Point this validation error at the field that actually supplied the output format.**

When `outputFormat` is nil and the effective value comes from `defaults.output_format`, this still reports `<section>.output_format` as the conflicting setting. That makes invalid `tui = true` configs harder to diagnose because the message names the wrong key.


<details>
<summary>💡 Suggested fix</summary>

```diff
 func validateWorkflowTUI(section string, defaults DefaultsConfig, outputFormat *string, tui *bool) error {
 	effectiveOutputFormat := outputFormat
+	outputField := fmt.Sprintf("%s.output_format", section)
 	if effectiveOutputFormat == nil {
 		effectiveOutputFormat = defaults.OutputFormat
+		outputField = "defaults.output_format"
 	}
 	if tui != nil && effectiveOutputFormat != nil && *tui && isExecJSONOutputFormat(*effectiveOutputFormat) {
 		return fmt.Errorf(
-			"workspace config %s.tui cannot be true when %s.output_format is %q or %q",
+			"workspace config %s.tui cannot be true when workspace config %s is %q or %q",
 			section,
-			section,
+			outputField,
 			model.OutputFormatJSONValue,
 			model.OutputFormatRawJSONValue,
 		)
 	}
 	return nil
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/workspace/config_validate.go` around lines 121 - 128, The error
message currently always references section+".output_format" even when
effectiveOutputFormat was taken from defaults; update the error to report the
actual field that supplied the value by checking outputFormat and
effectiveOutputFormat: if outputFormat != nil use section+".output_format",
otherwise use the defaults field (e.g. section+".defaults.output_format" or the
exact defaults key name used in your model) in the process of building the
fmt.Errorf string so the reported key matches the source of
effectiveOutputFormat; use the existing variables tui, effectiveOutputFormat,
and outputFormat to choose the correct field name.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:cfb4e507-6796-4f02-9ecd-afb099e21990 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `validateWorkflowTUI` always reports `<section>.output_format` even when the effective value came from `defaults.OutputFormat`.
  - Root cause: the error string does not track which field actually supplied the output format.
  - Intended fix: compute the source field name alongside the effective value and report the correct config key in the validation error.
  - Resolution: workflow TUI validation now reports the actual config field that supplied the conflicting output format.
