# Issue 5 - Review Thread Comment

**File:** `internal/core/tasks/parser.go:150`
**Date:** 2026-04-27 14:49:00 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Return `nil` when wrapping a nil error**

`WrapParseError` should preserve Go’s nil-error convention. Returning a wrapper for nil can produce false-positive failures.

<details>
<summary>Suggested fix</summary>

```diff
 func WrapParseError(path string, err error) error {
+	if err == nil {
+		return nil
+	}
 	return &ArtifactParseError{Path: path, Err: err}
 }
```

</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func WrapParseError(path string, err error) error {
	if err == nil {
		return nil
	}
	return &ArtifactParseError{Path: path, Err: err}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/tasks/parser.go` around lines 149 - 150, WrapParseError
currently returns an ArtifactParseError wrapper even when err is nil; change
WrapParseError to return nil when the incoming err is nil to preserve Go's
nil-error convention by checking err == nil and returning nil, otherwise return
&ArtifactParseError{Path: path, Err: err}; update any callers if they rely on
non-nil return values from WrapParseError.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:b27f5408-b987-4945-8b9c-5076daecce81 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Rationale: o wrapper de task parse tem a mesma violação de semântica de `nil` e precisa retornar `nil` quando não há erro subjacente.

## Resolve

Thread ID: `PRRT_kwDORy7nkc593g-t`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc593g-t
```

---

_Generated from PR review - CodeRabbit AI_
