# Issue 15 - Review Thread Comment

**File:** `sdk/compozy/config/yaml_loader_test.go:19`
**Date:** 2025-10-31 14:57:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Use t.Context() instead of context.Background() in tests.**

Per coding guidelines, tests should never use `context.Background()`; use `t.Context()` instead to enable proper cancellation and cleanup.



Apply this diff:

```diff
-	ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
+	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/config/yaml_loader_test.go around line 19, the test creates a
context with context.Background(); update this to use the test's cancellable
context by replacing context.Background() with t.Context() (i.e., ctx :=
logger.ContextWithLogger(t.Context(), logger.NewForTests())), ensuring the test
function has the standard *testing.T parameter named t so the t.Context() method
is available.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFEr`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFEr
```

---
*Generated from PR review - CodeRabbit AI*
