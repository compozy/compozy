---
status: resolved
file: internal/cli/makefile_publish_test.go
line: 25
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Rl8o,comment:PRRC_kwDORy7nkc621sgn
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

# Verify this file currently uses a single test flow and no t.Run subtests.
rg -nP '^\s*func\s+TestPublishExtensionSDKsTargetRequiresVerificationAndPublicAccess\s*\(t \*testing\.T\)\s*{' internal/cli/makefile_publish_test.go
rg -nP '\bt\.Run\("Should' internal/cli/makefile_publish_test.go || true
```

Repository: compozy/compozy

Length of output: 148

---



</details>

**Use `t.Run("Should...")` subtests with table-driven pattern.**

This test packs all assertions into a single flow without subtests. Per coding guidelines, convert to table-driven subtests using `t.Run("Should...")` and mark independent subtests with `t.Parallel()`.

<details>
<summary>Proposed refactor</summary>

```diff
 func TestPublishExtensionSDKsTargetRequiresVerificationAndPublicAccess(t *testing.T) {
 	makefile := readRepoMakefile(t)
-
-	if !strings.Contains(makefile, "publish-extension-sdks: verify build-extension-sdks") {
-		t.Fatalf("expected publish target to depend on verify and build-extension-sdks\nMakefile:\n%s", makefile)
-	}
-	for _, want := range []string{
-		"npm publish --workspace `@compozy/extension-sdk` --access public",
-		"npm publish --workspace `@compozy/create-extension` --access public",
-	} {
-		if !strings.Contains(makefile, want) {
-			t.Fatalf("expected Makefile to contain %q\nMakefile:\n%s", want, makefile)
-		}
-	}
+	tests := []struct {
+		name string
+		want string
+	}{
+		{"Should require verify and build-extension-sdks prerequisites", "publish-extension-sdks: verify build-extension-sdks"},
+		{"Should publish `@compozy/extension-sdk` publicly", "npm publish --workspace `@compozy/extension-sdk` --access public"},
+		{"Should publish `@compozy/create-extension` publicly", "npm publish --workspace `@compozy/create-extension` --access public"},
+	}
+
+	for _, tt := range tests {
+		tt := tt
+		t.Run(tt.name, func(t *testing.T) {
+			t.Parallel()
+			if !strings.Contains(makefile, tt.want) {
+				t.Fatalf("expected Makefile to contain %q\nMakefile:\n%s", tt.want, makefile)
+			}
+		})
+	}
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestPublishExtensionSDKsTargetRequiresVerificationAndPublicAccess(t *testing.T) {
	makefile := readRepoMakefile(t)
	tests := []struct {
		name string
		want string
	}{
		{"Should require verify and build-extension-sdks prerequisites", "publish-extension-sdks: verify build-extension-sdks"},
		{"Should publish `@compozy/extension-sdk` publicly", "npm publish --workspace `@compozy/extension-sdk` --access public"},
		{"Should publish `@compozy/create-extension` publicly", "npm publish --workspace `@compozy/create-extension` --access public"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(makefile, tt.want) {
				t.Fatalf("expected Makefile to contain %q\nMakefile:\n%s", tt.want, makefile)
			}
		})
	}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/makefile_publish_test.go` around lines 11 - 25, Refactor
TestPublishExtensionSDKsTargetRequiresVerificationAndPublicAccess into
table-driven subtests: keep the initial check for the publish target dependency
in its own t.Run("Should require verify and build-extension-sdks") (call
readRepoMakefile once) and then convert the loop over the expected npm publish
lines into a table of test cases where each case runs as
t.Run(fmt.Sprintf("Should contain %q", want), func(t *testing.T) { t.Parallel();
assert the strings.Contains(makefile, want) }), ensuring each subtest references
the same makefile variable and uses t.Parallel() for independent execution.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:9a6673c5-4ccb-4bd4-8184-cb9dc54bcba5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The test currently bundles multiple independent expectations into one flow, so a failure does not clearly identify which contract regressed.
  - Since issue 001 already requires restructuring this test, the remaining publish command assertions will be converted into table-driven `t.Run("Should ...")` subtests with `t.Parallel()` where the cases are independent.
  - Resolved by splitting the prerequisite and publish assertions into named subtests and running the independent cases in parallel.
