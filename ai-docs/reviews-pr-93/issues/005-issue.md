# Issue 5 - Review Thread Comment

**File:** `internal/setup/reusable_agent_sources.go:378`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Validate `ResolvedPath` before calling `filepath.Clean`.**

Line 364 currently turns an empty `ResolvedPath` into `"."`, so a malformed extension source falls back to the process CWD instead of failing fast. That can load or install the wrong directory entirely.

<details>
<summary>🐛 Proposed fix</summary>

```diff
-	resolvedPath := filepath.Clean(strings.TrimSpace(source.ResolvedPath))
-	if resolvedPath == "" {
+	resolvedPath := strings.TrimSpace(source.ResolvedPath)
+	if resolvedPath == "" {
 		return nil, "", fmt.Errorf("extension reusable agent source path is required")
 	}
+	resolvedPath = filepath.Clean(resolvedPath)
 	info, err := os.Stat(resolvedPath)
```

</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	resolvedPath := strings.TrimSpace(source.ResolvedPath)
	if resolvedPath == "" {
		return nil, "", fmt.Errorf("extension reusable agent source path is required")
	}
	resolvedPath = filepath.Clean(resolvedPath)
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, "", fmt.Errorf("stat extension reusable agent %q: %w", resolvedPath, err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("extension reusable agent %q is not a directory", resolvedPath)
	}

	parentDir := filepath.Dir(resolvedPath)
	sourceDir := filepath.Base(resolvedPath)
	return os.DirFS(parentDir), sourceDir, nil
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/reusable_agent_sources.go` around lines 364 - 378, Check and
reject an empty ResolvedPath before calling filepath.Clean: first trim
whitespace from source.ResolvedPath (reference: source.ResolvedPath) and if the
trimmed value is empty return the existing error (e.g., "extension reusable
agent source path is required") so we don't let filepath.Clean turn it into "."
and wrongly use the process CWD; then proceed to call filepath.Clean on the
non-empty trimmed path and continue with os.Stat, info.IsDir(), and the existing
parentDir/sourceDir logic.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4cdfae9c-22b0-4501-8a36-0aa965d55bf2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: now trims `ResolvedPath`, rejects empty input before `filepath.Clean`, and only then resolves the on-disk directory. The same root-cause fix was also applied to extension skill-pack source resolution.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVMD`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVMD
```

---

_Generated from PR review - CodeRabbit AI_
