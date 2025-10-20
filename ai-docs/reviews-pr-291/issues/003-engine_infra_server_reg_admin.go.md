# Issues for `engine/infra/server/reg_admin.go`

## Issue 3 - Review Thread Comment

**File:** `engine/infra/server/reg_admin.go:141`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Prefer typed query binding + validator; include allowed values in 400 details**

Current parsing works, but we can leverage gin binding and go-playground/validator to both shorten code and auto-validate.

- Option A (small change): include allowed values in details.

```diff
- router.RespondWithError(
-   c,
-   http.StatusBadRequest,
-   router.NewRequestError(http.StatusBadRequest, "invalid source parameter", nil),
- )
+ router.RespondWithError(
+   c,
+   http.StatusBadRequest,
+   router.NewRequestError(http.StatusBadRequest, "invalid source parameter", fmt.Errorf("allowed values: repo,builder")),
+ )
```

- Option B (preferred): use binding.

```go
type reloadQuery struct {
  Source string `form:"source" binding:"omitempty,oneof=repo builder"`
}
```

Then bind with c.ShouldBindQuery(&q) and map validation errors via RespondWithError. As per coding guidelines (validator usage, consistent 4xx mapping).

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/infra/server/reg_admin.go around lines 122 to 135, replace the manual
query parsing with gin's query binding and validator: define a small reloadQuery
struct with a Source field tagged `form:"source" binding:"omitempty,oneof=repo
builder"`, call c.ShouldBindQuery(&q), and when it returns a validation error
map it to router.RespondWithError returning http.StatusBadRequest and include
the allowed values ("repo", "builder") in the error details; if binding succeeds
return q.Source and true. If you prefer the minimal change, keep current logic
but change the BadRequest error details to explicitly list the allowed values in
the message.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP2`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP2
```

---
*Generated from PR review - CodeRabbit AI*
