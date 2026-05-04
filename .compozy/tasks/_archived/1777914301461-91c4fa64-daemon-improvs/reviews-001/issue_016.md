---
status: resolved
file: internal/core/run/journal/journal.go
line: 404
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:2209926a672e
review_hash: 2209926a672e
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 016: Consider consolidating duplicate terminal event classification.
## Review Comment

`isTerminalJournalEvent` duplicates the logic of `isTerminalEvent` (lines 768-778). Both check the same four event kinds (`RunCompleted`, `RunFailed`, `RunCancelled`, `RunCrashed`). If terminal event kinds change, both functions must be updated in sync.

Consider reusing `isTerminalEvent` in `recordDroppedSubmit` to avoid drift:

## Triage

- Decision: `valid`
- Root cause: `isTerminalJournalEvent` duplicates the exact terminal-event set already defined by `isTerminalEvent` in the same file, so terminal classification can drift if one helper changes without the other.
- Fix approach: reuse `isTerminalEvent` in the dropped-submit accounting path instead of keeping two terminal classifiers in sync.
- Resolution: `recordDroppedSubmit` now reuses `isTerminalEvent`, removing the duplicate terminal classifier.
- Regression coverage: `go test ./internal/cli ./internal/core/run/journal ./internal/daemon` passed after the journal cleanup.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
