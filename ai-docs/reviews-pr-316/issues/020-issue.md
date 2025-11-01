# Issue 20 - Review Thread Comment

**File:** `sdk/compozy/loader_test.go:123`
**Date:** 2025-11-01 01:57:03 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Replace `context.Background()` with test context**

This test uses `context.Background()`, but our test guidelines require deriving contexts from `t.Context()` (or the shared helpers like `lifecycleTestContext`) so configuration and logger state are wired properly. Please initialize the context via the lifecycle helper instead of `context.Background()`.

```diff
-	ctx := context.Background()
+	ctx := lifecycleTestContext(t)
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	ctx := t.Context()
	err := engine.loadFromDir(ctx, "", nil)
	assert.Error(t, err)
	engine = &Engine{}
	err = engine.loadFromDir(ctx, "", func(context.Context, string) error { return nil })
	assert.Error(t, err)
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/loader_test.go around lines 117 to 123, the test creates a
context with context.Background() which bypasses test lifecycle wiring; replace
ctx := context.Background() with a test-derived context (e.g. ctx :=
lifecycleTestContext(t) or ctx := t.Context() depending on available helper),
and add the necessary import or helper call if not already present so
configuration and logger state are correctly wired for the test.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2y`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2y
```

---
*Generated from PR review - CodeRabbit AI*
