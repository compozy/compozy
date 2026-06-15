---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/internal/acpshared/session_exec.go
line: 317
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6JqhqR,comment:PRRC_kwDORy7nkc7LlP3Z
---

# Issue 010: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Return explicit error for nil context in job-control handlers.**

The `Pause()` and `SendMessage()` methods fall back to `context.Background()` when receiving a nil context, severing cancellation and deadline propagation from callers. Per guidelines, avoid `context.Background()` outside `main` and focused tests; these methods cross runtime boundaries by calling external services like `CancelSession()`. Return an explicit error instead of silently substituting background context.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/internal/acpshared/session_exec.go` around lines 315 - 317,
The Pause() and SendMessage() methods in session_exec.go should not silently
fall back to context.Background() when receiving a nil context, as this breaks
cancellation and deadline propagation across runtime boundaries. Replace the
conditional check that assigns context.Background() to ctx with code that
returns an explicit error when ctx is nil. This ensures callers are notified of
invalid input rather than having their cancellation signals ignored when these
methods call external services like CancelSession().
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:19f10328bf780e05e60c1ad9 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `sessionTurnController.Pause` and `SendMessage` replace a nil caller context with `context.Background()`, detaching runtime-bound job-control calls from cancellation/deadline propagation.
- Fix approach: return an explicit error for nil contexts in both handlers and add focused tests for the invalid-input path. Regression coverage belongs in `internal/core/run/internal/acpshared/session_exec_test.go`, the existing session-turn controller suite, so it is a minimal test-only touch outside the initial code-file list.

## Resolution

- Resolved by rejecting nil job-control contexts with a sentinel error.
- Verification: `rtk make verify` exited 0 after the code changes.
