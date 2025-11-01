# Issue 13 - Review Thread Comment

**File:** `sdk/compozy/loader_test.go:123`
**Date:** 2025-11-01 12:25:24 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Resolution

- Split input validation checks into dedicated subtests and hardened expectations with `require.Error`.

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Split validation scenarios into separate subtests.**

This function tests two distinct input validation scenarios (nil engine and empty directory path) without subtests. Each validation case should be its own `t.Run("Should...")` subtest.



Example structure:

```go
func TestLoadFromDirValidatesInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should return error when engine is nil", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		var engine *Engine
		err := engine.loadFromDir(ctx, "", nil)
		require.Error(t, err)
	})

	t.Run("Should return error when directory path is empty", func(t *testing.T) {
		t.Parallel()
		ctx := lifecycleTestContext(t)
		engine := &Engine{}
		err := engine.loadFromDir(ctx, "", func(context.Context, string) error { return nil })
		require.Error(t, err)
	})
}
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/loader_test.go around lines 114 to 123, the test mixes two
distinct validation scenarios in one block; split them into separate t.Run
subtests so each case is isolated. Create one subtest "Should return error when
engine is nil" that calls t.Parallel(), builds ctx via lifecycleTestContext(t),
declares var engine *Engine and asserts an error from engine.loadFromDir(ctx,
"", nil); create a second subtest "Should return error when directory path is
empty" that calls t.Parallel(), builds ctx, constructs engine := &Engine{},
calls engine.loadFromDir(ctx, "", func(context.Context, string) error { return
nil }) and asserts an error (use require.Error for stricter checks).
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gNDTG`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gNDTG
```

---
*Generated from PR review - CodeRabbit AI*
