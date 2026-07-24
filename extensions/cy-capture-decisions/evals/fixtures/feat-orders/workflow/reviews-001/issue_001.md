---
status: resolved
file: internal/orders/eventstore.go
line: 42
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Writer not idempotent on the event stream

## Review Comment

Appending the same command twice produced duplicate events. Add an idempotency key on the write path.

## Triage

- Decision: `VALID`
- Notes: Fixed by keying appends on the command id; verified by the replay test.
