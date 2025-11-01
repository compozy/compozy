# Issue 17 - Review Thread Comment

**File:** `sdk/compozy/engine_loading_errors_test.go:42`
**Date:** 2025-10-31 14:57:19 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Remove unnecessary loop variable capture for Go 1.25.2.**

The `tc := tc` pattern is no longer needed in Go 1.22+ due to automatic per-iteration loop variable capture. Since this project uses Go 1.25.2, this line is unnecessary. Additionally, subtests should call `t.Parallel()` since the parent test does.



Apply this diff:

```diff
 	for _, tc := range tests {
-		tc := tc
 		t.Run(tc.name, func(t *testing.T) {
+			t.Parallel()
 			err := tc.call()
 			require.Error(t, err)
 			assert.Contains(t, err.Error(), "engine is nil")
 		})
 	}
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "engine is nil")
		})
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/engine_loading_errors_test.go around lines 40 to 42, remove the
unnecessary loop variable capture line `tc := tc` (Go 1.25.2 auto-captures
per-iteration variables) and inside the subtest function immediately call
`t.Parallel()` so each subtest runs in parallel with the parent; ensure the
subtest body then uses `tc` directly without reassigning it.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFE8`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFE8
```

---
*Generated from PR review - CodeRabbit AI*
