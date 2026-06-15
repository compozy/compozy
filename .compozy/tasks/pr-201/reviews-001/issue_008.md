---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/model/task_runtime.go
line: 83
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhp1,comment:PRRC_kwDORy7nkc7LlP21
---

# Issue 008: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Treat blank workflow qualifiers as unset during matching.**

Line 81 currently enforces workflow equality whenever `Workflow != nil`, including when `*Workflow` trims to `""`. That makes a whitespace-configured qualifier behave as “match only empty workflow” instead of “unscoped rule”.

<details>
<summary>💡 Proposed fix</summary>

```diff
 func (r TaskRuntimeRule) Matches(target TaskRuntimeTarget) bool {
-	if r.Workflow != nil && strings.TrimSpace(*r.Workflow) != strings.TrimSpace(target.Workflow) {
-		return false
-	}
+	if r.Workflow != nil {
+		workflow := strings.TrimSpace(*r.Workflow)
+		if workflow != "" && workflow != strings.TrimSpace(target.Workflow) {
+			return false
+		}
+	}
 	switch {
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if r.Workflow != nil {
		workflow := strings.TrimSpace(*r.Workflow)
		if workflow != "" && workflow != strings.TrimSpace(target.Workflow) {
			return false
		}
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/model/task_runtime.go` around lines 81 - 83, The condition in
the TaskRuntime matching logic (line 81-83) incorrectly treats whitespace-only
workflow qualifiers as set to empty string instead of unset. Currently, it
returns false whenever Workflow is not nil, even if it trims to empty string.
Fix this by adding an additional check: only enforce the workflow equality
comparison if the trimmed Workflow value is non-empty. Modify the condition to
check that after trimming, the Workflow is not empty before comparing it with
the target's workflow. This ensures blank/whitespace-only qualifiers are treated
as unscoped rules that match any workflow, rather than rules that match only
empty workflows.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:2ebaa1206929ca38c09b0beb -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TaskRuntimeRule.Matches` treats a non-nil whitespace workflow qualifier as a real empty-workflow constraint instead of an unset qualifier.
- Fix approach: trim the qualifier once and enforce workflow equality only when the trimmed qualifier is non-empty; add coverage for blank workflow qualifiers matching any workflow. Regression coverage belongs in `internal/core/model/model_test.go`, the existing runtime-config suite, so it is a minimal test-only touch outside the initial code-file list.

## Resolution

- Resolved by treating blank workflow qualifiers as unscoped.
- Verification: `rtk make verify` exited 0 after the code changes.
