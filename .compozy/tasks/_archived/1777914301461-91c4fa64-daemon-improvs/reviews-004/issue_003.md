---
status: resolved
file: internal/store/globaldb/registry.go
line: 911
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58kNgS,comment:PRRC_kwDORy7nkc656GNg
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't make case normalization a hard failure.**

`os.Stat` has already proven `resolvedPath` exists and is a directory. The new `ReadDir` walk adds a stronger permission requirement on every parent directory, so workspace roots under searchable-but-unreadable parents now fail here even though they previously normalized fine. Case canonicalization should be best-effort, not a new blocker.


<details>
<summary>💡 Suggested fallback</summary>

```diff
+import "io/fs"
...
 	canonicalPath, err := canonicalizeExistingPathCase(resolvedPath)
 	if err != nil {
+		if errors.Is(err, fs.ErrPermission) || os.IsPermission(err) {
+			return filepath.Clean(resolvedPath), nil
+		}
 		return "", fmt.Errorf("globaldb: canonicalize workspace path %q: %w", resolvedPath, err)
 	}
```
</details>


Also applies to: 946-949

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/registry.go` around lines 907 - 911, The current call
to canonicalizeExistingPathCase in registry.go should be made best-effort rather
than a hard failure: if canonicalizeExistingPathCase(resolvedPath) returns an
error, do not return that error — instead fall back to returning
filepath.Clean(resolvedPath) (optionally logging/debugging the canonicalization
error) so lack of read permission on parent directories doesn't block
normalization; apply the same change for the other instance referenced (the
similar block around lines 946-949) so both uses of canonicalizeExistingPathCase
degrade gracefully.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3b83986d-d641-4b98-9c1f-3d955d92a465 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `normalizeWorkspaceRoot` already resolves symlinks and `os.Stat`s the final directory before calling `canonicalizeExistingPathCase`, so the path is known-good at that point.
  - Root cause: the case-walk adds a stricter `ReadDir` requirement on every parent directory, which can fail even when the resolved workspace path is searchable and valid.
  - Fix approach: make the case-walk best-effort by falling back to the cleaned path when directory enumeration or case matching cannot continue, instead of turning canonicalization into a hard registration failure.
  - Resolution: `canonicalizeExistingPathCaseWith` now returns the cleaned absolute path when parent-directory reads or case matching fail, so canonicalization no longer blocks registration of already-validated workspace roots.
  - Regression coverage: `TestCanonicalizeExistingPathCaseWithFallsBackToCleanPathWhenParentsCannotBeRead` proves unreadable parents no longer fail normalization.
  - Verification: `go test ./internal/store/globaldb -run 'TestRegistryValidationBranches|TestCanonicalizeExistingPathCaseWithUsesOnDiskNames|TestCanonicalizeExistingPathCaseWithFallsBackToCleanPathWhenParentsCannotBeRead|TestGetByPathPrefersResolvedCanonicalWorkspaceRow' -count=1` passed. `make verify` also passed with `2548` tests, `2` skipped helper-process tests, and a successful `go build ./cmd/compozy`.
