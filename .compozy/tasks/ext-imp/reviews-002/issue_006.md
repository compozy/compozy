---
status: resolved
file: internal/core/run/exec/exec.go
line: 1240
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4094514795,nitpick_hash:61b4ff1d58a2
review_hash: 61b4ff1d58a2
source_review_id: "4094514795"
source_review_submitted_at: "2026-04-12T04:17:10Z"
---

# Issue 006: Consider protecting DryRun field from hook mutation.
## Review Comment

The validation protects state-defining fields, but `DryRun` also affects execution behavior significantly. If a hook could flip `DryRun` from true to false, it would enable real execution when the user expected dry-run.

## Triage

- Decision: `valid`
- Notes:
  - Root cause: `validateExecPreparedStateMutation` snapshots several state-defining fields after exec preparation, but it omits `DryRun`.
  - Impact: a mutable `run.pre_start` hook can flip `DryRun` from `true` to `false` after state preparation and unexpectedly turn a dry run into a real execution.
  - Fix approach: include `DryRun` in the protected snapshot/validation set and extend the exec mutation regression test coverage.
  - Implemented: added `dryRun` to the protected prepared-state snapshot and extended `internal/core/run/exec/exec_test.go` to reject post-preparation `dry_run` mutations.
  - Verification: focused exec tests passed and the final `make verify` run passed cleanly.
