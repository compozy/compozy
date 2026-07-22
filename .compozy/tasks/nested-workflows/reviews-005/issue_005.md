---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: pending
file: internal/core/worktree/review_isolation.go
line: 580
severity: high
author: claude-code
provider_ref:
---

# Issue 005: Pre-staged workflow artifacts poison review integration

## Review Comment

Isolation creation deliberately excludes the workflow artifact directory from the clean-source check and captures the existing index, so staged review/task artifacts are accepted. During integration, however, `validateStagedReviewIndex` scans the entire index and rejects every staged path not changed by the current batch instead of comparing the transaction-owned delta with `indexBackup`. An untouched staged artifact therefore makes auto-commit fail deterministically. The failure path at lines 386-388 reverses only the worktree patch after `git add`, leaving the rejected batch staged; subsequent batches then fail the unchanged-index check and the source is left with an index/worktree mismatch.

Validate only the batch's index delta relative to the captured baseline, preferably in a temporary index, and restore the exact initial batch-owned entries on any validation failure while preserving unrelated staging. Add coverage for an allowed pre-staged artifact and for validation failure with byte-identical index restoration.

## Triage

- Decision: `UNREVIEWED`
- Notes:
