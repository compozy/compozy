# Issue 7 - Review Thread Comment

**File:** `engine/tool/nativeuser/registry_test.go:81`
**Date:** 2025-11-01 01:57:01 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🔴 Critical_

**Replace `context.Background()` with `t.Context()` in test.**

Line 54 violates the testing guideline: "In tests, never use context.Background(); use t.Context() instead."

Apply this diff:

```diff
 func TestRegisterConcurrent(t *testing.T) {
 	Reset()
 	var wg sync.WaitGroup
-	ctx := context.Background()
+	ctx := t.Context()
 	errCh := make(chan error, 25)
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestRegisterConcurrent(t *testing.T) {
	Reset()
	var wg sync.WaitGroup
	ctx := t.Context()
	errCh := make(chan error, 25)
	for i := 0; i < 25; i++ {
		wg.Add(1)
		id := fmt.Sprintf("tool-%d", i)
		go func(toolID string) {
			defer wg.Done()
			h := func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
				return map[string]any{"id": toolID}, nil
			}
			errCh <- Register(toolID, h)
		}(id)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	ids := IDs()
	assert.Len(t, ids, 25)
	for _, id := range ids {
		def, ok := Lookup(id)
		require.True(t, ok)
		res, err := def.Handler(ctx, map[string]any{}, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, id, res["id"])
	}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/tool/nativeuser/registry_test.go around lines 51 to 81, the test
creates a context with context.Background() on line 54 which violates the
guideline; replace that with the test's context by changing ctx :=
context.Background() to ctx := t.Context() so the test uses the cancellable test
context provided by the testing framework.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2Y`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2Y
```

---
*Generated from PR review - CodeRabbit AI*
