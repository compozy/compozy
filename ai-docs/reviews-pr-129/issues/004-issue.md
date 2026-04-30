# Issue 4 - Review Thread Comment

**File:** `internal/core/reviews/parser.go:154`
**Date:** 2026-04-27 14:49:00 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Preserve nil-error semantics in `WrapParseError`**

`WrapParseError` currently returns a non-nil error even when `err` is nil. That can cause false validation failures downstream.

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

In `@internal/core/reviews/parser.go` around lines 153 - 154, WrapParseError
currently returns a non-nil *ArtifactParseError even when the input err is nil;
change WrapParseError to preserve nil-error semantics by returning nil if err ==
nil, otherwise return &ArtifactParseError{Path: path, Err: err}. Update the
function (WrapParseError) so callers receive nil when there is no underlying
error, referencing the ArtifactParseError type for the non-nil case.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:b27f5408-b987-4945-8b9c-5076daecce81 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Rationale: `WrapParseError(nil)` precisa respeitar a convenção de erro nulo em Go para não fabricar falhas inexistentes.

## Resolve

Thread ID: `PRRT_kwDORy7nkc593g-j`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc593g-j
```

---

_Generated from PR review - CodeRabbit AI_
