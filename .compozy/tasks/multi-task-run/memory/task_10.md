# Task 10 Memory

## V2: Update Documentation and End-to-End Coverage (FINAL TASK)

Closed out the worktree-backed parallel multi-run feature with accurate
user-facing docs and full-flow verification. Documentation-dominant change plus
two new test files; no production source changes (the runtime was complete after
task_09).

### What changed (docs)
- `README.md`
  - Removed all obsolete "parallel falls back to enqueued" wording.
  - Daemon Runtime Model bullet + quickstart now show `--parallel`/`--parallel-limit`.
  - `[tasks.run]` config example + supported-sections bullet gained
    `run_multiple_parallel_limit = 2`.
  - Config notes: valid modes `enqueued`/`parallel`, limit default `2` (must be > 0),
    and mode/limit precedence (flag > config > default).
  - `tasks run` flags table gained `--parallel` (bool) and `--parallel-limit` (int, 2).
  - Detailed multi-run section rewritten: worktree isolation (isolates working
    tree/index/HEAD; does NOT isolate ports/creds/quotas/caches), V1 preservation
    + non-goals (no auto-merge/push/branch/delete, no conflict prediction),
    fail-late aggregation (siblings continue; parent fails naming slugs; cancel
    cancels running + marks not-started canceled; exit non-zero on aggregate
    failure), and the `task multi-run handoff:` block shape.
- `docs/events.md`
  - New "## Multi-Run Events" section documenting all 8 `task.multi.*` kinds and
    every `kinds.TaskRunMultiplePayload` field (incl. additive `parallel_limit` +
    `worktree_path`/`base_branch`/`base_commit`/`worktree_status`).
  - Envelope kind count `51` -> `60` (52 prior were documented; +8 multi).

### What changed (tests)
- `pkg/compozy/events/docs_test.go` — added the 8 `task.multi.*` kinds to
  `TestEventsDocumentationEnumeratesAllPublicKinds` so the doc guards them.
- `pkg/compozy/events/kinds/docs_test.go` (new) —
  `TestTaskRunMultiplePayloadFieldsDocumented` reflects the payload's JSON tags
  and asserts each appears in `docs/events.md` (4 levels up from the kinds dir).
- `internal/cli/tasks_run_parallel_e2e_test.go` (new):
  - `TestREADMETasksRunSnippetsMatchCLIHelp` — every long flag the README shows on
    a `tasks run` line must exist on `newTasksRunCommandWithDefaults(nil, …)`;
    asserts `--parallel` (bool/false) + `--parallel-limit` (int/2) and that the
    obsolete fallback wording is gone.
  - `TestTasksRunMultipleParallelEndToEndReportsWorktreePaths` (git-gated) — full
    CLI -> in-process daemon -> real worktree allocation; asserts
    `mode=parallel`, the handoff block, and snapshot items carry worktree path
    (under `paths.WorktreesDir`) + `preserved` + base branch `main`.
  - `TestTasksRunMultipleParallelLimitOneEndToEnd` (git-gated) — `--parallel-limit 1`
    completes both children and the `task.multi.started` event carries
    `parallel_limit=1` (read via `ListRunEvents`).

### Key decisions
- e2e harness: `installInProcessCLIDaemonBootstrapWithConfigClient` with
  `RunManagerConfig{WorktreesRoot: paths.WorktreesDir, Prepare/Execute stubs}` +
  committed `git init -b main` workspace. Default `OpenRunScope`/`BuildRunID` are
  fine; only `Prepare`/`Execute` need stubbing (mirrors the daemon parallel tests).
  Resolve home + worktrees root BEFORE constructing the manager (call
  `prepareInProcessCLIDaemonHome` then `ResolveHomePaths`).
- The README↔help test ties snippets to the real command surface (not a brittle
  golden diff): forward direction = documented flags must exist.
- Daemon-level concurrency bounding is already proven
  (`...ParallelBoundsConcurrency`/`...Cancellation`); the CLI limit-1 test proves
  the flag plumbs through and the resolved limit is emitted, avoiding redundant
  timing assertions at the (blocking) `--stream` layer.

### Follow-up (out of scope)
- `--dry-run` parallel runs still create real git worktrees (the scheduler does
  not gate worktree allocation on `DryRun`; only child execution is skipped).

### Verification
- `make fmt` clean, `make lint` 0 issues, `make go-build` ok,
  `make test` 3669 pass / 3 skipped (pre-existing live-Codex + daemon-helper gates).
