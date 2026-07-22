---
provider: manual
pr:
round: 4
round_created_at: 2026-07-22T19:59:29Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 347
severity: high
author: codex
provider_ref:
---

# Issue 002: Concurrent source edits can enter generated review commits

## Review Comment

Review isolation checks source cleanliness only when the isolation is created. `Apply` later patches the live source tree, stages every batch-touched path with `git add -A`, and commits those paths without proving that they still match the isolation baseline. A user or editor can make a non-conflicting change to a touched file after startup; the generated patch applies, and line 347 stages both changes into Compozy's automated commit. `applyMu` serializes Compozy batches but cannot protect against external Git or filesystem activity. On a later failure, the raw index snapshot restoration can also discard staging changes made after the snapshot.

Fail closed when any touched source path or the relevant index state changed since isolation began. Restore the index with lock- and compare-and-swap-aware semantics instead of overwriting it blindly. Add regression coverage for an external same-file edit before apply and for concurrent staging during the failure rollback path.

## Triage

- Decision: `VALID`
- Notes: `Apply` currently validates neither the source paths named by the isolated patch nor the source index against the clean state captured by `NewReviewIsolation`. A non-conflicting external edit to a touched file therefore survives `git apply`, is swept into `git add -A`, and can enter the generated commit. The rollback path also restores a raw index snapshot with `os.WriteFile` without taking Git's index lock or checking that the index still matches the transaction-owned post-stage state, so concurrent staging can be lost. Preserve the isolation baseline, reject touched-path or index drift before applying, and restore the index only under `index.lock` after a compare-and-swap check. Regression tests exercise a same-file external edit through `Apply` and a real concurrent `git add` before the rollback compare-and-swap.
- Resolution: `Apply` now compares every touched source path to the seeded baseline, rejects source-index drift, validates staged entries against the isolated workspace, and restores a failed transaction's index only while holding `index.lock` and only when the current bytes still match the transaction-owned staged snapshot. Added real-Git regressions for non-conflicting same-file edits and concurrent staging during rollback. The full repository verification pipeline passed.
