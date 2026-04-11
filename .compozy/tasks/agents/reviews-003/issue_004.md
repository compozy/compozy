---
status: resolved
file: internal/core/run/exec/prompt_exec.go
line: 42
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4092869982,nitpick_hash:c25cbe939628
review_hash: c25cbe939628
source_review_id: "4092869982"
source_review_submitted_at: "2026-04-10T23:18:05Z"
---

# Issue 004: Wrap propagated errors with operation context.
## Review Comment

Several branches return raw errors (for example on Line 43, Line 48, Line 53, Line 79), which makes nested-failure diagnosis harder in callers and logs.

As per coding guidelines: `Prefer explicit error returns with wrapped context using fmt.Errorf("context: %w", err)`.

Also applies to: 46-49, 52-53, 77-80

## Triage

- Decision: `INVALID`
- Notes:
  - The raw returns in `ExecutePreparedPrompt` were checked against the current callees.
  - `agent.EnsureAvailable` already returns a structured `AvailabilityError` (or `ErrRuntimeConfigNil`), `prepareExecRunState`/`state.writeStarted` already wrap their operational failures, and `newExecRuntimeJobWithMCP` already wraps prompt-write failures.
  - Adding another `execute prepared prompt: ...` layer on top of those errors would mostly duplicate existing context and make the nested-agent failure strings noisier without exposing a missing root cause.
  - The one materially missing error-preservation bug in this function is the separate `issue_005` branch, which is handled independently.
