---
provider: manual
pr:
round: 7
round_created_at: 2026-07-23T00:22:01Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 402
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Successful review commits can be reported as failed

## Review Comment

`commitReviewPatch` runs `git commit --only` and then recaptures the source index with the caller's context. If that context is canceled between the successful commit and `captureGitIndex`, or the index read otherwise fails, the commit has already landed but `Apply` returns an error. The runner consequently marks the review batch failed and skips status-finalized/provider events even though the source branch contains the remediation commit. This is an unknown outcome that can cause operators or automation to retry work that is already committed.

Make the successful commit the durable boundary: refresh or reconcile the index with a non-canceled bounded context, and do not return an ordinary reversible-integration failure after Git has confirmed the commit. Preserve enough state to distinguish and recover a committed-but-unreconciled result. Add a deterministic regression seam that fails index refresh after commit and asserts that the landed commit is reported and finalized exactly once.

## Triage

Valid. `commitReviewPatch` completes `git commit --only`, which durably advances the source repository's `HEAD`, and then calls `captureGitIndex` with the caller's context. Cancellation or an index-read error at that point is returned from `Apply` even though rollback is no longer possible. `afterReviewJobSuccess` only emits review finalization and provider events after `Apply` succeeds, so the durable commit is misclassified as a failed batch and may be retried.

Fixed the post-commit boundary in `ReviewIsolation`: retain the staged index as the expected post-commit state, refresh through a non-canceled bounded context, and record a failed refresh for semantic reconciliation before the next integration without returning failure for the already-landed commit. A per-isolation capture seam and real-Git regression test force only the post-commit refresh to fail, verify the bounded context survives caller cancellation, and confirm the batch commit lands exactly once across recovery. The focused regression and complete `internal/core/worktree` suite pass.
