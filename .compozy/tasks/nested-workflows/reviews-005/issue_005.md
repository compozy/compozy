---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: resolved
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

- Decision: `VALID`
- Notes: `NewReviewIsolation` intentionally captures the source index after excluding workflow artifacts from the clean-source check, so pre-existing staged artifacts are part of the accepted baseline. During auto-commit, `validateStagedReviewIndex` instead compared every staged path with the batch path set; an untouched baseline artifact therefore failed validation even though the index had not changed outside the transaction. That error path called only `rollbackReviewWorktree`, leaving entries written by `git add` in the source index. The remediation now constructs the expected post-stage index from the captured baseline, compares the real index with that expected transaction result, and restores the captured index through the existing compare-and-swap rollback on validation failure. Regression coverage proves successful integration with an unrelated pre-staged artifact and byte-identical index restoration after a forced validation mismatch. Full verification uses a short `COMPOZY_HOME` for frontend E2E because this review worktree's path exceeds macOS Unix-socket limits; Go commands run with `COMPOZY_HOME` unset.`
