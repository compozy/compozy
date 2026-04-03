# Issue 5 - Review Thread Comment

**File:** `internal/core/workspace/config.go:91`
**Date:** 2026-04-03 18:11:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cd /repo && git ls-files | head -20
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
# Find the file mentioned in the review
fd "config.go" --path "*workspace*"
```

Repository: compozy/compozy

Length of output: 274

---

🏁 Script executed:

```shell
# Get file size and content
wc -l internal/core/workspace/config.go
```

Repository: compozy/compozy

Length of output: 96

---

🏁 Script executed:

```shell
# Read the entire file to understand context
cat -n internal/core/workspace/config.go
```

Repository: compozy/compozy

Length of output: 8959

---



</details>

**Resolve symlinks during workspace discovery to walk the real directory tree.**

Line 88 uses `filepath.Abs()` but does not resolve symlinks. When the start path is a symlink, upward traversal will walk the symlink path instead of the real path, potentially missing the workspace marker.

<details>
<summary>Suggested fix</summary>

```diff
 absStart, err := filepath.Abs(resolvedStart)
 if err != nil {
 	return "", fmt.Errorf("resolve workspace start dir: %w", err)
 }
+ absStart, err = filepath.EvalSymlinks(absStart)
+ if err != nil {
+ 	return "", fmt.Errorf("resolve workspace start dir symlinks: %w", err)
+ }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	absStart, err := filepath.Abs(resolvedStart)
	if err != nil {
		return "", fmt.Errorf("resolve workspace start dir: %w", err)
	}
	absStart, err = filepath.EvalSymlinks(absStart)
	if err != nil {
		return "", fmt.Errorf("resolve workspace start dir symlinks: %w", err)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/workspace/config.go` around lines 88 - 91, The code uses
filepath.Abs(resolvedStart) to compute absStart but doesn't resolve symlinks;
update the logic in the workspace discovery (around resolvedStart and absStart)
to call filepath.EvalSymlinks on the absolute path (e.g., first compute absPath
:= filepath.Abs(resolvedStart), then realStart, err :=
filepath.EvalSymlinks(absPath)) and use realStart for upward traversal and
workspace marker checks so the real directory tree is walked; preserve existing
error wrapping (fmt.Errorf) on EvalSymlinks failures.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:84952ff0-22fa-465c-8136-1b9d835c0c64 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Restated change: resolve symlinks before upward workspace discovery so traversal follows the real directory tree.
- Evidence: `Discover` currently uses `filepath.Abs` only; if the starting directory is a symlink into a workspace subtree, parent traversal walks the symlink path instead of the real workspace path and can miss `.compozy/`.

## Resolve

Thread ID: `PRRT_kwDORy7nkc54yeIm`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc54yeIm
```

---
*Generated from PR review - CodeRabbit AI*
