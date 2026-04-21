---
status: resolved
file: internal/api/core/interfaces.go
line: 56
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:cc3f7606305a
review_hash: cc3f7606305a
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 003: Prefer request structs over positional ID parameters.
## Review Comment

Methods like `WorkflowMemoryFile(ctx, workspaceRef, workflowSlug, fileID)`, `TaskDetail(ctx, workspaceRef, workflowSlug, taskID)`, and `ReviewDetail(ctx, workspaceRef, workflowSlug, round, issueRef)` pass multiple string/primitive identifiers positionally, where a misordered argument silently compiles. Small, dedicated request structs would make these browser-facing reads self-documenting and safer to extend without future signature churn, aligning with the guideline: "Design small, focused interfaces; accept interfaces, return structs."

## Triage

- Decision: `invalid`
- Notes:
  - This is an internal transport/query interface design preference, not a demonstrated correctness or safety defect in the current implementation.
  - Current call sites are limited, named by surrounding context, and already covered by compile-time interface checks and transport tests.
  - Converting these signatures to request structs would cascade through multiple handlers and services outside the scoped batch without fixing an observed bug, so this batch will close it as analysis-only.

## Resolution

- Closed as analysis-only. No production change was made because the comment identifies an interface-style preference rather than a concrete defect within the scoped batch.
- Verified with `make verify`.
