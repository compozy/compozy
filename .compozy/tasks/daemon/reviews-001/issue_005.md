---
status: resolved
file: internal/api/core/handlers.go
line: 644
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm2,comment:PRRC_kwDORy7nkc65HKYA
---

# Issue 005: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Reject non-positive `round` values here too.**

`GetReviewRound` and `ListReviewIssues` already enforce a positive round, but `FetchReview` forwards `body.Round` unchecked. Accepting `0` or negative values here makes the review API inconsistent and pushes bad input deeper into the service layer.


<details>
<summary>Suggested fix</summary>

```diff
  workspace, ok := h.requireWorkspaceRef(c, body.Workspace)
  if !ok {
  	return
  }
+ if body.Round != nil && *body.Round <= 0 {
+ 	h.respondError(c, validationProblem(
+ 		"round_invalid",
+ 		"round must be a positive integer",
+ 		map[string]any{"field": "round"},
+ 	))
+ 	return
+ }
 
  result, err := h.Reviews.Fetch(c.Request.Context(), workspace, c.Param("slug"), ReviewFetchRequest{
  	Workspace: workspace,
  	Provider:  strings.TrimSpace(body.Provider),
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers.go` around lines 630 - 644, The Fetch review
handler currently forwards body.Round unchecked into h.Reviews.Fetch; add a
validation after requireWorkspaceRef and before constructing ReviewFetchRequest
that rejects non-positive rounds (body.Round <= 0) and returns an HTTP 400 Bad
Request (use the existing error helper such as h.errorBadRequest or the
project's standard response helper) with a clear message like "round must be a
positive integer"; keep existing flow using reviewFetchBody, h.bindJSON,
requireWorkspaceRef, and ReviewFetchRequest and only proceed to call
h.Reviews.Fetch when the round is valid.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c1d7e4c5-68cf-4aef-a285-1de9756bb650 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `FetchReview` forwards `body.Round` without validating it, unlike the round-specific review endpoints that already reject non-positive values.
- Fix plan: validate `body.Round` after workspace resolution, return the standard `round_invalid` validation problem for `<= 0`, and add a handler error-path test.
- Resolution: Implemented and verified with `make verify`.
