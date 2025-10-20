# Issues for `engine/llm/memory_integration.go`

## Issue 10 - Review Thread Comment

**File:** `engine/llm/memory_integration.go:122`
**Date:** 2025-10-20 03:07:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Retrieve logger from context instead of passing it as a parameter.**

The function accepts `logger.Logger` as a parameter, which violates the coding guideline: "Use logger.FromContext(ctx); never pass a logger as a parameter or via DI."



As per coding guidelines.

Apply this diff to retrieve the logger from context:

```diff
 func resolveWritableMemory(
+	ctx context.Context,
-	log logger.Logger,
 	memories map[string]Memory,
 	ref core.MemoryReference,
 ) (Memory, bool) {
+	log := logger.FromContext(ctx)
 	memory, exists := memories[ref.ID]
 	if !exists {
 		log.Debug("Memory reference not found; skipping", "memory_id", ref.ID)
 		return nil, false
 	}
 	if ref.Mode == core.MemoryModeReadOnly {
 		log.Debug("Skipping read-only memory", "memory_id", ref.ID)
 		return nil, false
 	}
 	return memory, true
 }
```

Update the call site at line 72:

```diff
-		memory, ok := resolveWritableMemory(log, memories, ref)
+		memory, ok := resolveWritableMemory(ctx, memories, ref)
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func resolveWritableMemory(
	ctx context.Context,
	memories map[string]Memory,
	ref core.MemoryReference,
) (Memory, bool) {
	log := logger.FromContext(ctx)
	memory, exists := memories[ref.ID]
	if !exists {
		log.Debug("Memory reference not found; skipping", "memory_id", ref.ID)
		return nil, false
	}
	if ref.Mode == core.MemoryModeReadOnly {
		log.Debug("Skipping read-only memory", "memory_id", ref.ID)
		return nil, false
	}
	return memory, true
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/memory_integration.go around lines 107 to 122, the function
resolveWritableMemory currently takes a logger.Logger parameter which violates
the guideline to retrieve loggers from context; change the signature to accept a
context.Context instead of logger.Logger, call logger.FromContext(ctx) inside
the function to obtain the logger, and replace uses of the passed-in log with
that logger; also update the call site at line 72 to pass the existing context
(instead of a logger) when invoking resolveWritableMemory.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP-`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP-
```

---
*Generated from PR review - CodeRabbit AI*


---

## Issue 11 - Review Thread Comment

**File:** `engine/llm/memory_integration.go:142`
**Date:** 2025-10-20 03:07:21 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Retrieve logger from context instead of passing it as a parameter.**

The function accepts `logger.Logger` as a parameter despite already having `context.Context` available. This violates the coding guideline: "Use logger.FromContext(ctx); never pass a logger as a parameter or via DI."



As per coding guidelines.

Apply this diff to retrieve the logger from context:

```diff
 func appendMemoryBatch(
 	ctx context.Context,
 	memory Memory,
 	memoryID string,
 	batch []Message,
-	log logger.Logger,
 	errs *[]error,
 ) {
+	log := logger.FromContext(ctx)
 	if err := memory.AppendMany(ctx, batch); err != nil {
 		log.Error(
 			"Failed to append messages to memory atomically",
 			"memory_id", memoryID,
 			"error", err,
 		)
 		*errs = append(*errs, fmt.Errorf("failed to append messages to memory %s: %w", memoryID, err))
 		return
 	}
 	log.Debug("Messages stored atomically in memory", "memory_id", memoryID)
 }
```

Update the call site at line 76:

```diff
-		appendMemoryBatch(ctx, memory, ref.ID, batch, log, &errs)
+		appendMemoryBatch(ctx, memory, ref.ID, batch, &errs)
```


> Committable suggestion skipped: line range outside the PR's diff.

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyQA`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyQA
```

---
*Generated from PR review - CodeRabbit AI*
