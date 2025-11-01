# Issue 2 - Review Thread Comment

**File:** `engine/project/indexer.go:257`
**Date:** 2025-11-01 01:57:00 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🔴 Critical_

**Use `groupCtx` instead of `ctx` in the goroutine.**

The goroutine should use `groupCtx` (from `errgroup.WithContext` at line 112) rather than the parent `ctx` parameter to ensure proper cancellation propagation when any goroutine in the group fails.



Apply this diff:

```diff
 		group.Go(func() error {
-			return p.putResourceWithMeta(ctx, store, metaSources, keyCopy, toolCopy)
+			return p.putResourceWithMeta(groupCtx, store, metaSources, keyCopy, toolCopy)
 		})
```

Note: This same issue appears in all other `indexProject*` functions (memories, schemas, models, embedders, vectorDBs, knowledgeBases) and should be fixed consistently.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		group.Go(func() error {
			return p.putResourceWithMeta(groupCtx, store, metaSources, keyCopy, toolCopy)
		})
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
engine/project/indexer.go around lines 255-257: the goroutine currently calls
p.putResourceWithMeta using the parent ctx; replace ctx with groupCtx so the
goroutine inherits the errgroup cancellation context (i.e., call
p.putResourceWithMeta(groupCtx, store, metaSources, keyCopy, toolCopy) inside
the group.Go). Apply the same change to all other indexProject* functions
(memories, schemas, models, embedders, vectorDBs, knowledgeBases) so every
goroutine uses groupCtx instead of the parent ctx.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2M`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2M
```

---
*Generated from PR review - CodeRabbit AI*
