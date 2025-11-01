# Issue 6 - Review Thread Comment

**File:** `engine/tool/nativeuser/registry_test.go:26`
**Date:** 2025-11-01 01:57:01 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🔴 Critical_

**Replace `context.Background()` with `t.Context()` in tests.**

Line 15 violates the testing guideline: "In tests, never use context.Background(); use t.Context() instead."

Apply this diff:

```diff
 func TestRegisterAndLookup(t *testing.T) {
 	Reset()
-	ctx := context.Background()
+	ctx := t.Context()
 	h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
 		return map[string]any{"ok": true}, nil
 	}
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestRegisterAndLookup(t *testing.T) {
	Reset()
	ctx := t.Context()
	h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}
	require.NoError(t, Register("test-tool", h))
	def, ok := Lookup("test-tool")
	require.True(t, ok)
	assert.Equal(t, "test-tool", def.ID)
	res, err := def.Handler(ctx, map[string]any{}, map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"ok": true}, res)
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/tool/nativeuser/registry_test.go around lines 13 to 26, the test uses
context.Background() (line 15); replace that call with t.Context() so the test
uses the test's cancellable/context tied lifecycle. Update the ctx variable
initialization to use t.Context() and run the test to ensure it still passes.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2W`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2W
```

---
*Generated from PR review - CodeRabbit AI*
