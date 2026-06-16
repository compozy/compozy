# Task Number TUI Ledger

## Goal (incl. success criteria):

Fix execution TUI task cards so PRD task jobs display the real workflow/spec task number from `task_N.md`, not the selected-list index. Success requires targeted regression coverage, `make verify` passing, and `cy-impl-peer-review` reaching SHIP.

## Constraints/Assumptions:

- Prefix shell commands with `rtk`.
- Do not use destructive git commands or revert unrelated tracked files.
- Preserve existing uncommitted UI styling/sidebar changes.
- Root cause fix only: derive job `TaskNumber` from canonical task entry name, not UI inference.

## Key decisions:

- Accepted plan persisted at `.codex/plans/20260614-214447-task-number.md`.
- Owning invariant is in `internal/core/plan`: prepared PRD task jobs must carry canonical task number.

## State:

Completed. Implementation patched for write-side and read-side legacy event paths; targeted tests passed; `cy-impl-peer-review` round 6 returned SHIP; final fresh `make verify` passed.

## Done:

- Root cause traced: `CodeFile` strips `.md`, so current `batchTaskNumber(..., batchFiles)` returns 0.
- Confirmed sidebar already honors `taskNumber > 0`.
- Persisted accepted plan and set active goal.
- Patched `batchTaskNumber` to parse from `IssueEntry.Name`.
- Added regression assertions in plan preparation and UI queue handling.
- `rtk go test ./internal/core/plan ./internal/core/run/ui -run 'TestPrepareJobsForPRDTasksForcesSingleBatchPerTask|TestPreparePRDTasksUsesSharedRunArtifactsWithoutChangingTaskOrder|TestHandleJobQueuedStoresTaskMetadata|TestSidebarCardUsesOfficialTaskNumber' -count=1` passed.
- User showed latest TUI still rendering 01/02/03; inspected real `advos` artifacts and confirmed latest `job.queued` events still omit `task_number`.
- Added `tasks.ExtractTaskIdentityNumber`, daemon snapshot backfill, and UI live-event backfill guarded by task metadata.
- `rtk go test ./internal/core/tasks ./internal/core/plan ./internal/core/run/ui ./internal/daemon -run 'TestExtractTaskIdentityNumber|TestPrepareJobsForPRDTasksForcesSingleBatchPerTask|TestPreparePRDTasksUsesSharedRunArtifactsWithoutChangingTaskOrder|TestHandleJobQueuedStoresTaskMetadata|TestHandleJobQueuedBackfillsLegacyTaskNumber|TestHandleJobQueuedDoesNotBackfillWithoutTaskMetadata|TestSidebarCardUsesOfficialTaskNumber|TestRunSnapshotBuilderCoversLifecycleBranches' -count=1` passed.
- Fresh `rtk make verify` passed with exit code 0 after all code changes.
- `cy-impl-peer-review` rounds 1-5 were environment failures only (daemon version mismatch, temp daemon port collision, or Claude auth under temp HOME).
- Built a temporary version-matched CLI at `/tmp/compozy-peer-cli` to attach to the existing authenticated daemon without stopping active runs.
- `cy-impl-peer-review` round 6 completed with verdict `SHIP`: 0 blockers, 3 risks, 2 nits. Summary artifact: `.peer-reviews/20260615T005454Z/impl-review-summary-round6.md`.
- Final fresh raw verification passed: `rtk zsh -lc 'make verify > .peer-reviews/20260615T005454Z/final-make-verify-raw.log 2>&1; ...'` exited 0 with Go tests `DONE 3556 tests, 4 skipped`, Playwright `5 passed`, and `All verification checks passed`.

## Now:

Prepare final handoff.

## Next:

None.

## Open questions (UNCONFIRMED if needed):

None.

## Working set (files/ids/commands):

- `internal/core/plan/prepare.go`
- `internal/core/plan/prepare_test.go`
- `internal/core/run/ui/update_test.go`
- `internal/core/run/ui/update.go`
- `internal/core/tasks/parser.go`
- `internal/core/tasks/parser_test.go`
- `internal/daemon/run_snapshot.go`
- `internal/daemon/run_snapshot_test.go`
- `.peer-reviews/20260615T005454Z/impl-review-summary-round6.md`
- `rtk go test ...`
- `rtk make verify`
