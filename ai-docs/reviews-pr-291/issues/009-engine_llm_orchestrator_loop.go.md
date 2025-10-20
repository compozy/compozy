# Issues for `engine/llm/orchestrator/loop.go`

## Issue 12 - Review Thread Comment

**File:** `engine/llm/orchestrator/loop.go:306`
**Date:** 2025-10-20 03:07:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Use role constants for consistency.**

Prefer roleUser (or llmadapter.RoleUser) over hardcoded "user" here to keep role handling uniform.

```diff
-    llmadapter.Message{Role: "user", Content: hint},
+    llmadapter.Message{Role: roleUser, Content: hint},
```


> Committable suggestion skipped: line range outside the PR's diff.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/orchestrator/loop.go around lines 293 to 306, the code appends a
message using the hardcoded role string "user"; replace that with the canonical
role constant (e.g. llmadapter.RoleUser or roleUser) to ensure consistency
across the codebase, update or add the necessary import/reference if not
present, and run a quick build to confirm no missing identifier errors.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyQB`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyQB
```

---
*Generated from PR review - CodeRabbit AI*
