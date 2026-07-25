---
provider: manual
pr:
round: 10
round_created_at: 2026-07-25T01:52:47Z
status: resolved
file: internal/daemon/task_multi.go
line: 2730
severity: critical
author: claude-code
provider_ref:
---

# Issue 001: Child artifact sync overwrites concurrent parent edits

## Review Comment

`syncTaskMultiGroupChildArtifacts` writes a completed child's operational
directory back with `worktree.OverlayTree(source, destination)`. `OverlayTree`
now exact-replaces the destination tree, but this path never checks whether the
canonical parent directory changed after it was mirrored into the child
worktree.

The run manager does not serialize task, review, and memory writers per task
group. A review-fix run, direct task-group run, Host API memory write, or human
edit can therefore update the canonical task-group directory while the parallel
child is active. When the child finishes, its launch-time snapshot replaces the
whole directory: newer task state can be reverted, resolved review issues can
be reopened or deleted, and new memory files can disappear. Two overlapping
task-group runs can likewise race, with the last finisher silently winning.
These are first-class human-owned artifacts, so this is unrecoverable data loss,
not only stale display state.

Capture a baseline fingerprint or snapshot when mirroring the task group, then
perform a three-way/CAS reconciliation at write-back. If the canonical tree
drifted, merge non-conflicting child changes and fail with the worktree
preserved on conflicts; do not replace the parent tree unconditionally. Add a
deterministic regression that changes a parent review or memory file after
child launch and proves completion preserves both the parent edit and the
child's task update.

## Triage

- Decision: `VALID`
- Notes:
  - `startTaskMultiWorktreeChild` mirrors the canonical task-group artifacts
    into the child worktree but retains no snapshot of the mirrored tree.
    `taskWorktreeChildRun` therefore carries only the allocation and remapped
    runtime configuration into settlement.
  - After a successful child run, `syncTaskMultiGroupChildArtifacts` calls
    `worktree.OverlayTree` directly. Because `OverlayTree` exact-replaces the
    destination, any review, memory, task, or human edit made in the canonical
    directory after launch is silently removed or reverted.
  - The fix will capture the child's launch-time artifact tree immediately
    after mirroring, carry that baseline with the child run, and reconcile the
    baseline, current canonical tree, and completed child tree per path.
    Non-conflicting changes will be combined; divergent changes to the same
    path or canonical drift during installation will fail the child settlement
    and preserve its worktree.
  - Regression coverage will exercise the real parallel task-group lifecycle:
    a canonical memory edit made after child launch must survive alongside the
    child's task update. A conflict case will verify that divergent edits do
    not mutate the canonical tree and retain the child worktree for recovery.
  - Scoped checks pass: `go vet ./internal/daemon`, targeted
    `golangci-lint`, and the artifact-sync integration tests under `go test
    -race` are clean. The full repository gate
    `COMPOZY_HOME=/tmp/compozy-review-verify.uxZwIh make verify` completed
    frontend lint, typecheck, tests, build, and Go formatting, then stopped on
    the pre-existing unrelated `gocyclo` violation at
    `internal/cli/daemon_commands.go:1958`
    (`resolveTaskPresentationMode`, complexity 16 > 15). That file is identical
    to `HEAD`, has no worktree diff, and is outside this batch's code scope.
