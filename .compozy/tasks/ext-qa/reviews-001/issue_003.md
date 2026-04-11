---
status: pending
file: internal/cli/workspace_config_test.go
line: 216
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Tdvy,comment:PRRC_kwDORy7nkc624Dt1
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Use table-driven subtests with `t.Run("Should...")` for these new cases.**

Line 152 and Line 185 add two near-duplicate top-level tests. Please fold them into one table-driven test with `t.Run("Should ...")` cases to align with repo test policy and avoid duplication.

<details>
<summary>♻️ Proposed refactor</summary>

```diff
-func TestApplyWorkspaceDefaultsUsesStartPresentationOverrides(t *testing.T) {
-	t.Parallel()
-	...
-}
-
-func TestApplyWorkspaceDefaultsUsesFixReviewsPresentationOverrides(t *testing.T) {
-	t.Parallel()
-	...
-}
+func TestApplyWorkspaceDefaultsUsesWorkflowPresentationOverrides(t *testing.T) {
+	t.Parallel()
+
+	cases := []struct {
+		name       string
+		kind       commandKind
+		mode       core.Mode
+		section    string
+		wantFormat string
+	}{
+		{
+			name:       "start section overrides defaults",
+			kind:       commandKindStart,
+			mode:       core.ModePRDTasks,
+			section:    "start",
+			wantFormat: "json",
+		},
+		{
+			name:       "fix_reviews section overrides defaults",
+			kind:       commandKindFixReviews,
+			mode:       core.ModePRReview,
+			section:    "fix_reviews",
+			wantFormat: "raw-json",
+		},
+	}
+
+	for _, tc := range cases {
+		tc := tc
+		t.Run("Should "+tc.name, func(t *testing.T) {
+			t.Parallel()
+			// setup + assertions per tc...
+		})
+	}
+}
```
</details>

As per coding guidelines `**/*_test.go`: "MUST use t.Run("Should...") pattern for ALL test cases" and "Use table-driven tests with subtests (`t.Run`) as the default pattern".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/workspace_config_test.go` around lines 152 - 216, Combine the
two near-duplicate tests
TestApplyWorkspaceDefaultsUsesStartPresentationOverrides and
TestApplyWorkspaceDefaultsUsesFixReviewsPresentationOverrides into a single
table-driven test that iterates cases with t.Run("Should ...") subtests; for
each case include a name, the commandKind (commandKindStart or
commandKindFixReviews), core mode, the workspace TOML (with defaults + the
per-command section), expected outputFormat and expected tui value, then inside
each subtest recreate the temp root and startDir, call writeCLIWorkspaceConfig,
build state via newCommandState and cmd via newTestCommand, call chdirCLITest
and state.applyWorkspaceDefaults, and assert state.outputFormat and state.tui
match expectations (using the same checks currently present). Reference
functions/values:
TestApplyWorkspaceDefaultsUsesStartPresentationOverrides/TestApplyWorkspaceDefaultsUsesFixReviewsPresentationOverrides,
writeCLIWorkspaceConfig, newCommandState, newTestCommand, chdirCLITest, and
state.applyWorkspaceDefaults, and assert on state.outputFormat and state.tui.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:d6e338d6-3b52-41b5-a5f7-5db8d622ca4a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
