---
status: resolved
file: test/skills_bundle_test.go
line: 106
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCb,comment:PRRC_kwDORy7nkc60y0gg
---

# Issue 018: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Wrap the new test case in `t.Run("Should...")` to satisfy test conventions.**

This test is currently a single top-level block; please wrap it in a named subtest using the required `Should...` style.

<details>
<summary>Suggested adjustment</summary>

```diff
 func TestEmbeddedSkillsFSMatchesOnDisk(t *testing.T) {
 	t.Parallel()
+	t.Run("Should match embedded skills filesystem with on-disk filtered skills tree", func(t *testing.T) {
+		t.Parallel()
 
-	root := repoRoot(t)
-	source := filepath.Join(root, "skills")
-	sourceTree := snapshotTree(t, source)
+		root := repoRoot(t)
+		source := filepath.Join(root, "skills")
+		sourceTree := snapshotTree(t, source)
 
-	// Filter out non-skill files (embed.go, autoresearch artifacts, etc.)
-	wantTree := make(map[string]string, len(sourceTree))
-	for p, content := range sourceTree {
-		if strings.HasSuffix(p, ".go") {
-			continue
-		}
-		if strings.Contains(p, "autoresearch-") {
-			continue
-		}
-		wantTree[p] = content
-	}
+		// existing assertions...
+	})
 }
```
</details>

As per coding guidelines, `**/*_test.go`: "MUST use t.Run("Should...") pattern for ALL test cases".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestEmbeddedSkillsFSMatchesOnDisk(t *testing.T) {
	t.Parallel()
	t.Run("Should match embedded skills filesystem with on-disk filtered skills tree", func(t *testing.T) {
		t.Parallel()

		root := repoRoot(t)
		source := filepath.Join(root, "skills")
		sourceTree := snapshotTree(t, source)

		// Filter out non-skill files (embed.go, autoresearch artifacts, etc.)
		wantTree := make(map[string]string, len(sourceTree))
		for p, content := range sourceTree {
			if strings.HasSuffix(p, ".go") {
				continue
			}
			if strings.Contains(p, "autoresearch-") {
				continue
			}
			wantTree[p] = content
		}

		embeddedTree := make(map[string]string)
		err := fs.WalkDir(skills.FS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, readErr := fs.ReadFile(skills.FS, p)
			if readErr != nil {
				return readErr
			}
			embeddedTree[p] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("walk embedded FS: %v", err)
		}

		if len(embeddedTree) != len(wantTree) {
			t.Fatalf("expected embedded FS to contain %d files, got %d", len(wantTree), len(embeddedTree))
		}
		for p, wantContent := range wantTree {
			gotContent, ok := embeddedTree[p]
			if !ok {
				t.Fatalf("expected embedded FS to contain %s", p)
			}
			if gotContent != wantContent {
				t.Fatalf("expected embedded content for %s to match on-disk source", p)
			}
		}
	})
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@test/skills_bundle_test.go` around lines 56 - 106, Wrap the body of the
TestEmbeddedSkillsFSMatchesOnDisk test in a named subtest using t.Run with the
"Should..." naming convention (e.g., t.Run("Should match embedded skills FS with
on-disk source", func(t *testing.T) { ... })), moving t.Parallel() and the
existing logic into that closure so the top-level
TestEmbeddedSkillsFSMatchesOnDisk only calls t.Run and the assertions remain
unchanged.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:11b7943a-be71-43f5-bd60-8a6b15200679 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: `TestEmbeddedSkillsFSMatchesOnDisk` still executes its assertions directly at top level instead of wrapping the body in a `t.Run("Should ...")` subtest, which conflicts with the repository's test-structure rule.
- Fix plan: Wrap the body in a named `Should...` subtest and keep the existing assertions intact.
- Resolution: Wrapped the embedded-skills filesystem test body in a `Should...` subtest without changing its assertions. `make verify` passed.
