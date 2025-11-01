# Issue 21 - Review Thread Comment

**File:** `sdk/compozy/mode_test.go:30`
**Date:** 2025-11-01 01:57:03 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Refactor test to use isolated subtests per coding guidelines.**

This test violates the coding guideline requiring `t.Run("Should describe behavior", ...)` subtests for each behavior. It tests three distinct behaviors (cleanup invocation, nil handling, error handling) in a single function and reuses state/counter variables, creating test interdependence. Additionally, it's missing `t.Parallel()` for consistency.



Apply this refactor to follow the subtest pattern and ensure test isolation:

```diff
 func TestModeRuntimeStateCleanup(t *testing.T) {
+	t.Parallel()
+
+	t.Run("Should invoke cleanup functions exactly once and ignore nil entries", func(t *testing.T) {
+		t.Parallel()
-	state := &modeRuntimeState{}
-	counter := 0
-	state.addCleanup(func(context.Context) error {
-		counter++
-		return nil
-	})
-	state.addCleanup(nil)
-	err := state.cleanup(t.Context())
-	assert.NoError(t, err)
-	assert.Equal(t, 1, counter)
+		state := &modeRuntimeState{}
+		counter := 0
+		state.addCleanup(func(context.Context) error {
+			counter++
+			return nil
+		})
+		state.addCleanup(nil)
+		err := state.cleanup(t.Context())
+		assert.NoError(t, err)
+		assert.Equal(t, 1, counter)
+	})
 
+	t.Run("Should increment counter when cleanup returns error", func(t *testing.T) {
+		t.Parallel()
+		state := &modeRuntimeState{}
+		counter := 0
-	state.addCleanup(func(context.Context) error {
-		counter++
-		return errors.New("failure")
-	})
-	state.cleanupOnError(t.Context())
-	assert.Equal(t, 2, counter)
+		state.addCleanup(func(context.Context) error {
+			counter++
+			return errors.New("failure")
+		})
+		state.cleanupOnError(t.Context())
+		assert.Equal(t, 1, counter)
+	})
 }
```

As per coding guidelines.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestModeRuntimeStateCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should invoke cleanup functions exactly once and ignore nil entries", func(t *testing.T) {
		t.Parallel()
		state := &modeRuntimeState{}
		counter := 0
		state.addCleanup(func(context.Context) error {
			counter++
			return nil
		})
		state.addCleanup(nil)
		err := state.cleanup(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, 1, counter)
	})

	t.Run("Should increment counter when cleanup returns error", func(t *testing.T) {
		t.Parallel()
		state := &modeRuntimeState{}
		counter := 0
		state.addCleanup(func(context.Context) error {
			counter++
			return errors.New("failure")
		})
		state.cleanupOnError(t.Context())
		assert.Equal(t, 1, counter)
	})
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/mode_test.go around lines 12 to 30, the test combines three
distinct behaviors into one function and reuses state/counter leading to
interdependent assertions and missing parallelization; refactor into isolated
subtests using t.Run for each behavior ("invokes cleanup", "ignores nil
cleanup", "cleanupOnError invokes and returns error behavior") and call
t.Parallel() inside each subtest (or at top-level test if appropriate), create
fresh modeRuntimeState and counter variables per subtest, register the relevant
cleanup functions, invoke cleanup or cleanupOnError, and assert expected results
independently so tests no longer share state or order-dependent side effects.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa20`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa20
```

---
*Generated from PR review - CodeRabbit AI*
