---
status: resolved
file: internal/core/extension/host_api.go
line: 86
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:f3533abf08cf
review_hash: f3533abf08cf
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 025: Avoid defaulting to context.Background() in production code.
## Review Comment

Per coding guidelines, `context.Background()` should be avoided outside `main` and focused tests. Consider returning an error when `ctx` is nil instead of silently substituting a background context.

As per coding guidelines: "Pass `context.Context` as the first argument to all functions crossing runtime boundaries; avoid `context.Background()` outside `main` and focused tests".

## Triage

- Decision: `invalid`
- Notes: `HostAPIRouter.Handle` is invoked from the runtime request path with a live request context; there is no demonstrated production caller passing `nil`. The existing `context.Background()` fallback is a nil-safety guard and aligns with the package's broader `contextError(nil) == nil` behavior. Converting `nil` into a request failure would be a behavioral change without fixing a real defect in the current batch scope.
