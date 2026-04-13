---
status: resolved
file: internal/core/run/internal/acpshared/reusable_agent_lifecycle.go
line: 96
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4092828776,nitpick_hash:54fb530d878b
review_hash: 54fb530d878b
source_review_id: "4092828776"
source_review_submitted_at: "2026-04-10T22:56:04Z"
---

# Issue 006: Context fallback to context.Background() for nil input.
## Review Comment

While the coding guidelines recommend avoiding `context.Background()` outside `main` and focused tests, this fallback is a defensive measure to prevent panics if a caller passes nil. Consider whether callers should be required to provide a valid context instead, which would surface bugs earlier.

## Triage

- Decision: `invalid`
- Notes:
  - The `context.Background()` fallback is deliberate constructor-boundary normalization for a nil `cfg.Context`, not a workaround inside business logic.
  - Go allows callers to pass a nil `context.Context` interface value; normalizing once in `NewSessionUpdateHandler` prevents downstream panics in lifecycle emission and shutdown code without changing the semantics for valid callers.
  - No concrete bug, leak, or incorrect cancellation behavior was reproduced from this fallback, so there is no root-cause fix to make here.
  - Resolution: analysis complete; no code change required.
