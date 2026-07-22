---
provider: manual
pr:
round: 4
round_created_at: 2026-07-22T19:59:29Z
status: resolved
file: pkg/compozy/events/docs_test.go
line: 15
severity: low
author: codex
provider_ref:
---

# Issue 006: Public stalled and parked events are absent from the event inventory

## Review Comment

The public package exports `job.stalled` and `job.parked` with structured payloads, but `docs/events.md` has no event-contract sections for either kind. `TestEventsDocumentationEnumeratesAllPublicKinds` manually maintains a supposedly complete list and fixed count, yet omits both constants, so the completeness gate passes while public consumers cannot discover their payloads or lifecycle semantics.

Document both events and their payload fields alongside the other job lifecycle events, add them to the public inventory test, and derive the expected inventory from one authoritative exported-kind registry so future additions cannot bypass the documentation gate by forgetting to edit both the list and its hard-coded count.

## Triage

- Decision: `VALID`
- Notes: `EventKindJobStalled` and `EventKindJobParked` are exported from
  `pkg/compozy/events/event.go`, with public payloads in
  `pkg/compozy/events/kinds/job.go`, but the job-event section in
  `docs/events.md` documents neither contract. The documentation test duplicates
  selected constants in a hand-maintained slice and then only checks that slice's
  hard-coded length, so adding both constants without updating the test left the
  gate green. The assertion also accepts incidental inline mentions, which already
  exist for these names in the multi-run summary, instead of requiring event
  contract headings. Replace the duplicate slice and count with type-checked
  discovery of every exported package constant whose type is `EventKind`, require
  a contract heading for each kind, then document both missing lifecycle contracts.
  `docs/events.md` is outside the listed code-file scope but is the product contract
  named by the issue, so the minimum required documentation edit is included.
- Verification: `make verify` passed after the implementation. Its Bun command was
  given a short `COMPOZY_HOME` because this generated review-worktree path exceeds
  macOS's Unix-domain socket path limit; the Go checks retained their normal
  environment and the repository's complete verification dependency graph ran.
