---
status: resolved
file: internal/api/client/runs.go
line: 203
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:87c879f20e2e
review_hash: 87c879f20e2e
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 004: Wrap transcript failures with endpoint context.
## Review Comment

The new method returns raw errors from `doJSON` and `Decode()`, so callers can't tell whether the failure came from transcript loading or another run API path. Add transcript-specific context before returning them.

As per coding guidelines, "Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`".

## Triage

- Decision: `VALID`
- Notes: `GetRunTranscript` returned raw request/decode errors without endpoint context. Wrapped both `doJSON` and decode failures with transcript-specific run ID context using `%w`.
