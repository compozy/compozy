# Issue 11 - Review Thread Comment

**File:** `sdk/client/workflow.go:60`
**Date:** 2025-10-31 14:57:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Consider extracting common execution logic.**

`ExecuteWorkflow` and `ExecuteWorkflowSync` share nearly identical structure with only path suffix and expected status code differing. This duplication also appears in `sdk/client/task.go`. Consider extracting a shared helper across both files to reduce duplication and improve maintainability.

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFEa`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFEa
```

---
*Generated from PR review - CodeRabbit AI*
