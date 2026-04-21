---
status: resolved
file: internal/store/globaldb/registry.go
line: 925
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58kNgZ,comment:PRRC_kwDORy7nkc656GNo
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`In Go's standard library, what does filepath.Clean("") return?`

💡 Result:

filepath.Clean("") returns the string ".".

Citations:

- 1: https://pkg.go.dev/path/filepath
- 2: https://pkg.go.dev/path/filepath@go1.26.1
- 3: https://www.geeksforgeeks.org/go-language/filepath-clean-function-in-golang-with-examples/
- 4: https://pkg.go.dev/path/filepath@go1.16
- 5: https://pkg.go.dev/path/filepath@go1.26.0
- 6: https://go.dev/src/path/filepath/path.go

---

🏁 Script executed:

```shell
cat -n internal/store/globaldb/registry.go | sed -n '920,930p'
```

Repository: compozy/compozy

Length of output: 447

---

🏁 Script executed:

```shell
cat -n internal/store/globaldb/registry.go | sed -n '910,950p'
```

Repository: compozy/compozy

Length of output: 1383

---



</details>

**Reorder validation to catch whitespace-only input.**

`filepath.Clean("")` returns `"."`, so the current code allows whitespace-only paths like `"   "` to bypass the empty-path check and return `"."` instead of the intended error.

<details>
<summary>💡 Proposed fix</summary>

```diff
 func canonicalizeExistingPathCaseWith(
 	path string,
 	readDir func(string) ([]os.DirEntry, error),
 ) (string, error) {
-	cleanPath := filepath.Clean(strings.TrimSpace(path))
-	if cleanPath == "" {
+	trimmed := strings.TrimSpace(path)
+	if trimmed == "" {
 		return "", errors.New("globaldb: workspace path is required")
 	}
+	cleanPath := filepath.Clean(trimmed)
 	if !filepath.IsAbs(cleanPath) {
 		return cleanPath, nil
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("globaldb: workspace path is required")
	}
	cleanPath := filepath.Clean(trimmed)
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/registry.go` around lines 922 - 925, The validation
currently runs filepath.Clean before checking for empty input, letting
whitespace-only strings become "."; change the order to trim whitespace first
and validate that result is non-empty (use strings.TrimSpace on path and check
trimmed == ""), then call filepath.Clean on the trimmed value and assign to
cleanPath; update the error path that returns errors.New("globaldb: workspace
path is required") to trigger on the trimmed empty string.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3b83986d-d641-4b98-9c1f-3d955d92a465 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The current implementation does `filepath.Clean(strings.TrimSpace(path))` before validating emptiness, so whitespace-only input becomes `"."` and skips the required-path error.
  - Root cause: emptiness is checked after `filepath.Clean`, which normalizes empty strings into the current-directory marker.
  - Fix approach: trim first, reject `trimmed == ""`, then call `filepath.Clean(trimmed)`.
  - Resolution: `canonicalizeExistingPathCaseWith` now validates the trimmed input before calling `filepath.Clean`, so whitespace-only values correctly fail with `globaldb: workspace path is required`.
  - Regression coverage: `TestRegistryValidationBranches` now asserts that whitespace-only input returns an error instead of normalizing to `"."`.
  - Verification: `go test ./internal/store/globaldb -run 'TestRegistryValidationBranches|TestCanonicalizeExistingPathCaseWithUsesOnDiskNames|TestCanonicalizeExistingPathCaseWithFallsBackToCleanPathWhenParentsCannotBeRead|TestGetByPathPrefersResolvedCanonicalWorkspaceRow' -count=1` passed. `make verify` also passed with `2548` tests, `2` skipped helper-process tests, and a successful `go build ./cmd/compozy`.
