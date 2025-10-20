# Issues for `engine/llm/factory/metrics/metrics.go`

## Issue 9 - Review Thread Comment

**File:** `engine/llm/factory/metrics/metrics.go:65`
**Date:** 2025-10-20 03:07:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Name fallback should be a named constant.**

Replace the string literal "unknown" with a package constant for clarity and reuse.

Apply this diff:

```diff
 const (
   labelFactoryType = "factory_type"
   labelName        = "name"
   unitSeconds      = "s"
+  defaultNameUnknown = "unknown"
 )
@@
   finalName := strings.TrimSpace(name)
   if finalName == "" {
-    finalName = "unknown"
+    finalName = defaultNameUnknown
   }
```


> Committable suggestion skipped: line range outside the PR's diff.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/factory/metrics/metrics.go around lines 61 to 65, the string
literal "unknown" used as a fallback should be replaced with a package-level
named constant; add a const (e.g., const unknownFactoryType = "unknown") at the
top of the package and replace any direct uses of the literal with that constant
(and update the fallback logic to assign unknownFactoryType when finalType is
empty), ensuring all occurrences in this file/package use the new constant for
clarity and reuse.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP9`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP9
```

---
*Generated from PR review - CodeRabbit AI*
