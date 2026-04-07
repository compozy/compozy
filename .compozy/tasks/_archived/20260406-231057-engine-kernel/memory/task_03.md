# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement task 03 journal writer in `internal/core/run/journal/` with writer-owned seq assignment, append-before-publish semantics, bounded submit backpressure, batch flush, terminal fsync, metrics, and required tests.

## Important Decisions
- Resolved the task 03/task 07 dependency cycle by implementing the minimal real public replay surface in `pkg/compozy/runs/` (`RunSummary`, `Open`, `Summary`, `Replay`) instead of introducing a task-local scanner helper that task 07 would later replace.
- Kept journal durability semantics strict: events are encoded into the batch, flushed+synced, and only then published to the event bus, so live subscribers only observe persisted events.

## Learnings
- `golangci-lint run --fix` rewrites British spellings like `"cancelled"` to `"canceled"` in string literals, so reader-side terminal-status logic must not rely on dual literal switch cases or verification will reintroduce a duplicate-case compile error.
- The journal’s crash-recovery test seam works well when injected between `bufio.Writer.Flush()` and `file.Sync()`: tests can append a synthetic partial final line there and validate reader tolerance against the real on-disk contract.

## Files / Surfaces
- `.compozy/tasks/engine-kernel/task_03.md`
- `.compozy/tasks/engine-kernel/_techspec.md`
- `.compozy/tasks/engine-kernel/_tasks.md`
- `.compozy/tasks/engine-kernel/adrs/adr-003.md`
- `.compozy/tasks/engine-kernel/adrs/adr-004.md`
- `.compozy/tasks/engine-kernel/adrs/adr-006.md`
- `.compozy/tasks/engine-kernel/memory/MEMORY.md`
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/scanner.go`
- `pkg/compozy/runs/run_test.go`
- `internal/core/run/journal/journal.go`
- `internal/core/run/journal/journal_test.go`

## Errors / Corrections
- Initial journal loop implementation passed targeted tests but failed `make verify` on `gocyclo`; refactoring into active-loop/drain-loop helpers preserved behavior and satisfied lint.
- Full verification exposed that the misspell auto-fixer collapsed `"cancelled"` into `"canceled"` and recreated a duplicate switch case; reader terminal-status detection was rewritten to normalize spellings without dual literals.

## Ready for Next Run
- Task 03 is implemented and verified. Task 05 can now wire `journal.Submit` into executor/logging paths, and task 07 can extend the existing `pkg/compozy/runs/` foundation with `List`, `Tail`, `WatchWorkspace`, and dependency additions.
