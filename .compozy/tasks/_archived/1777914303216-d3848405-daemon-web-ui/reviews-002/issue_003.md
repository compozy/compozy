---
status: resolved
file: internal/api/httpapi/dev_proxy_test.go
line: 60
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:8dd1e8945d87
review_hash: 8dd1e8945d87
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 003: not applicable
## Review Comment

However, the codebase already demonstrates a cleaner pattern in other test files: save the previous mode and restore it in a cleanup function (as seen in `handlers_error_paths_test.go` and `browser_middleware_test.go`). Apply this same pattern for consistency:

```go
func TestDevProxyRoutesServeFrontendRequests(t *testing.T) {
t.Parallel()

previousMode := gin.Mode()
gin.SetMode(gin.TestMode)
t.Cleanup(func() { gin.SetMode(previousMode) })
```

Also applies to: 143-146, 199-202, 214-217

## Triage

- Decision: `invalid`
- Notes:
  - `gin` mode is global process state, and this package already has many parallel tests that call `gin.SetMode(gin.TestMode)` without per-test restoration.
  - Restoring the previous mode only in `dev_proxy_test.go` would not meaningfully improve isolation and can actually reintroduce inter-test coupling by flipping the global mode while sibling parallel tests are still constructing engines.
  - Analysis complete: no code change was made because the right fix would require a package-wide serialization strategy, not a one-file cleanup wrapper.
