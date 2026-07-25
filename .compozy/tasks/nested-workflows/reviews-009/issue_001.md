---
provider: manual
pr:
round: 9
round_created_at: 2026-07-24T20:14:29Z
status: resolved
file: internal/daemon/task_multi.go
line: 2464
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Task-group children update a disposable artifact alias

## Review Comment

For a parallel task-group child, `mirrorTaskMultiGroupArtifacts` first copies the
canonical initiative tree (including
`_task_groups/TG-NNN`) into the worktree, then creates a second copy at the
slug-shaped adapter path `<initiative>/TG-NNN`
(`internal/daemon/task_multi_artifacts.go:71-86`). This line selects that adapter
copy as `RuntimeConfig.TasksDir`, while `remapTaskMultiExecutionScope` leaves
`ExecutionScope.TasksDir`, `OperationalDir`, `ReviewsDir`, and `MemoryDir`
pointing at the canonical `_task_groups/TG-NNN` tree.

That split is observable by the agent. `plan.Prepare` discovers task entries
from `RuntimeConfig.TasksDir`, so `IssueEntry.AbsPath` points at the adapter.
`buildTaskFilesSection` then labels that adapter file as the exact task-tracking
path to update (`internal/core/prompt/prd.go:214-216`), and workflow memory is
also prepared relative to the adapter via
`memory.Prepare(cfg.TasksDir, ...)` (`internal/core/plan/prepare.go:805`).

Nothing reconciles those adapter artifacts with the canonical task-group tree.
`finishTaskMultiWorktreeChild` removes every successful child worktree
(`internal/daemon/task_multi.go:2604-2612,2685-2692`), and the task-artifact
write-back helper is owned by the inner parallel-task orchestrator, not this
parallel task-group queue. The result branch can therefore contain the agent's
source commit and the parent can report the child completed while the completed
task status, checklist updates, and memory are discarded. The canonical task
remains pending, so later review/completion gates cannot succeed and a relaunch
can execute the same task again.

Use the remapped `ExecutionScope.TasksDir` as the child runtime task directory
for task-group launches and remove the adapter copy, or add an explicit,
failure-aware adapter-to-canonical and canonical-to-parent reconciliation before
cleanup. Add an integration test that runs the real preparation path, updates
the exact task path supplied in the generated prompt, settles the child, and
asserts that the canonical parent `task_NN.md` retains the completed state.

## Triage

- Decision: `VALID`
- Notes: `remapTaskMultiChildRuntime` unconditionally derives `RuntimeConfig.TasksDir`
  from the slug, so a task-group child prepares prompts and workflow memory from
  `<initiative>/TG-NNN` even though its remapped `ExecutionScope` identifies
  `<initiative>/_task_groups/TG-NNN` as the canonical mutable tree. The outer
  parallel task-group queue then removes a successful child worktree without
  copying those ignored runtime artifacts to the parent workspace. The fix will
  use the remapped execution-scope task directory for task-group preparation and
  reconcile the child's canonical operational tree to the parent before cleanup.
  Reconciliation errors will fail the outer child item and preserve its worktree
  instead of discarding the only updated copy. Regression coverage will exercise
  the real `plan.Prepare` prompt path and both successful and failed reconciliation.
- Verification: The focused regression suite passes under the race detector:
  `go test -race ./internal/daemon -run
  'TestRemapTaskMultiChildRuntimeClonesTaskGroupExecutionScope|TestRunManagerTaskMultiGroupParallelPersistsCanonicalTaskArtifacts|TestRunManagerTaskMultiGroupParallelPreservesWorktreeWhenArtifactSyncFails'
  -count=1`. The required `make verify` gate stops in `make lint` on the pre-existing
  `internal/cli/daemon_commands.go:1958` `gocyclo` finding (complexity 16, limit
  15). That function is byte-for-byte identical to `HEAD` and outside this batch's
  sole production-file scope, so it was not modified or suppressed. The issue
  remains `valid`, not `resolved`, and no commit was created because verification
  was not clean.
