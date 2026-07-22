---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-prd/references/user-stories-template.md
line: 45
severity: high
author: claude-code
provider_ref:
---

# Issue 009: Unknown outcomes lack a durable recovery state machine

## Review Comment

The edge-case sweep asks generic interruption and repetition questions but does not recognize uncertain command outcomes or require a durable recovery model. A generated task can return `UNKNOWN_OUTCOME` without specifying when execution or replay is safe after transport loss, restart, incomplete persistence, fingerprint mismatch, or corruption.

For uncertain-outcome requirements, require explicit states for no record, pending/incomplete, completed success, completed failure, fingerprint mismatch, and corrupt/unreadable records. Each state must define whether execution may begin or repeat, the client response, durable evidence used, and retry behavior after restart or transport failure. Generate tests for deterministic completed-result replay, documented incomplete-record recovery, mismatch rejection without execution/replay, transport loss after commit, process restart, and returning unknown only when durable evidence cannot determine the result.

## Triage

- Decision: `VALID`
- Notes: The template's interruption and repetition probes mention connection loss, restart, partial completion, retry, and replay only as independent questions. They do not require a generated story to distinguish durable record states or to define execution, response, evidence, and retry behavior for an uncertain command outcome. This gap can leave `UNKNOWN_OUTCOME` underspecified after transport loss or restart. Fix the root cause in the edge-case sweep by adding a dedicated uncertain-outcome recovery probe with the required state matrix and downstream test obligations.
