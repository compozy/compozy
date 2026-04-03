# Issue 6 - Review Thread Comment

**File:** `test/release_config_test.go:102`
**Date:** 2026-04-03 18:11:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Use subtests (`t.Run`) for each case in these loops.**

This test currently runs multiple cases without `t.Run`, which violates the repository’s required test pattern and makes failures less isolated.



<details>
<summary>Proposed refactor</summary>

```diff
 func TestGoReleaserConfigUsesReadableChangelogTitlesAndFiltersReleaseCommits(t *testing.T) {
 	t.Parallel()
@@
 	expectedTitles := []string{
@@
 	}
 
 	for _, title := range expectedTitles {
-		if !strings.Contains(text, title) {
-			t.Fatalf("expected goreleaser changelog config to include readable group title %q", title)
-		}
+		title := title
+		t.Run("Should include readable title "+title, func(t *testing.T) {
+			t.Parallel()
+			if !strings.Contains(text, title) {
+				t.Fatalf("expected goreleaser changelog config to include readable group title %q", title)
+			}
+		})
 	}
@@
 	for _, title := range unexpectedTitles {
-		if strings.Contains(text, title) {
-			t.Fatalf("expected goreleaser changelog config to avoid emoji-only group title %q", title)
-		}
+		title := title
+		t.Run("Should avoid emoji-only title "+title, func(t *testing.T) {
+			t.Parallel()
+			if strings.Contains(text, title) {
+				t.Fatalf("expected goreleaser changelog config to avoid emoji-only group title %q", title)
+			}
+		})
 	}
@@
 	for _, filter := range expectedFilters {
-		if !strings.Contains(text, filter) {
-			t.Fatalf(
-				"expected goreleaser changelog config to exclude release automation commits with filter %q",
-				filter,
-			)
-		}
+		filter := filter
+		t.Run("Should exclude release automation filter "+filter, func(t *testing.T) {
+			t.Parallel()
+			if !strings.Contains(text, filter) {
+				t.Fatalf(
+					"expected goreleaser changelog config to exclude release automation commits with filter %q",
+					filter,
+				)
+			}
+		})
 	}
 }
```
</details>

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases" and "Use table-driven tests with subtests (`t.Run`) as the default pattern".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestGoReleaserConfigUsesReadableChangelogTitlesAndFiltersReleaseCommits(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	text := string(content)

	expectedTitles := []string{
		`title: "🎉 Features"`,
		`title: "🐛 Bug Fixes"`,
		`title: "⚡ Performance Improvements"`,
		`title: "🔒 Security"`,
		`title: "📚 Documentation"`,
		`title: "♻️ Refactoring"`,
		`title: "📦 Dependencies"`,
		`title: "🧪 Testing"`,
		`title: "Other Changes"`,
	}

	for _, title := range expectedTitles {
		title := title
		t.Run("Should include readable title "+title, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(text, title) {
				t.Fatalf("expected goreleaser changelog config to include readable group title %q", title)
			}
		})
	}

	unexpectedTitles := []string{
		`title: "\U0001F389"`,
		`title: "\U0001F41B"`,
		`title: "⚡"`,
		`title: "\U0001F510"`,
		`title: "\U0001F4DA"`,
		`title: "\U0001F527"`,
		`title: "\U0001F4E6"`,
		`title: "\U0001F9EA"`,
		`title: "\U0001F504"`,
	}

	for _, title := range unexpectedTitles {
		title := title
		t.Run("Should avoid emoji-only title "+title, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(text, title) {
				t.Fatalf("expected goreleaser changelog config to avoid emoji-only group title %q", title)
			}
		})
	}

	expectedFilters := []string{
		`- "^ci\\(release\\): "`,
		`- "^chore\\(release\\): "`,
	}

	for _, filter := range expectedFilters {
		filter := filter
		t.Run("Should exclude release automation filter "+filter, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(text, filter) {
				t.Fatalf(
					"expected goreleaser changelog config to exclude release automation commits with filter %q",
					filter,
				)
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

In `@test/release_config_test.go` around lines 43 - 102, The test
TestGoReleaserConfigUsesReadableChangelogTitlesAndFiltersReleaseCommits
currently iterates plain loops over expectedTitles, unexpectedTitles and
expectedFilters; refactor each loop into table-driven subtests using t.Run so
each case is isolated (e.g., for expectedTitles range run t.Run(title, func(t
*testing.T) { t.Parallel(); if !strings.Contains(text, title) { t.Fatalf(... )
}})), do the same for unexpectedTitles (assert absence) and expectedFilters,
preserve the existing assertion messages, and keep the outer test's t.Parallel()
as appropriate.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1a115553-53e2-4caa-908d-943f4b4e0142 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Restated change: convert the repeated assertion loops in `TestGoReleaserConfigUsesReadableChangelogTitlesAndFiltersReleaseCommits` into subtests.
- Evidence: the test currently ranges directly over multiple case lists without `t.Run`, which makes failures less isolated and does not follow the repository’s default subtest pattern.

## Resolve

Thread ID: `PRRT_kwDORy7nkc54yeIp`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc54yeIp
```

---
*Generated from PR review - CodeRabbit AI*
