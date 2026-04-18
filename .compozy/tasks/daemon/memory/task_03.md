# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Completed `task_03`: moved per-run operational state into `~/.compozy/runs/<run-id>/run.db`, wired the serialized journal writer to SQLite-backed durable storage, preserved compatibility artifacts during migration, and finished with a passing `make verify`.

## Important Decisions
- `internal/store/rundb` is the primary operational store for canonical events, job state, transcript rows, hook audit rows, token usage, and artifact sync history.
- The journal now writes to `run.db` before flushing the compatibility `events.jsonl` mirror so later replay/snapshot work can rely on SQLite ordering.
- Persisted run readers and exec resume paths keep a migration seam: prefer workspace-local metadata when present, otherwise resolve the home-scoped run directory.
- Public/extension-facing run metadata now includes `RunDBPath` where runtime contracts expose `RunArtifacts`.

## Learnings
- `pkg/compozy/runs.Open()` needed to preserve the already-resolved persisted run base directory; otherwise home-scoped runs silently fell back to workspace-local `events.jsonl`.
- Parallel plan/CLI/exec tests can collide on auto-generated run ids when they share the real home directory; test helpers now isolate `$HOME` or force stable per-test run ids.
- CLI/help/README surfaces needed explicit updates to `~/.compozy/runs/<run-id>/` once persisted exec allocation moved out of the workspace.

## Files / Surfaces
- `internal/store/rundb/{migrations.go,run_db.go,migrations_test.go,run_db_test.go}`
- `internal/core/run/journal/{journal.go,journal_test.go}`
- `internal/core/model/{artifacts.go,run_scope.go,model_test.go}`
- `internal/core/run/exec/{exec.go,exec_test.go,exec_integration_test.go}`
- `internal/core/extension/{audit.go,runtime.go}`
- `internal/core/kernel/handlers.go`
- `pkg/compozy/runs/{layout/layout.go,run.go,run_test.go}`
- `internal/core/plan/prepare_test.go`
- `internal/cli/{commands.go,root_command_execution_test.go,root_test.go,testdata/exec_help.golden}`
- `sdk/extension/types.go`
- `README.md`

## Errors / Corrections
- Fixed persisted exec and CLI tests that still discovered runs under `workspace/.compozy/runs`.
- Fixed `pkg/compozy/runs.Open()` so resolved home-scoped run paths are retained when metadata omits explicit artifact paths.
- Refactored `internal/store/rundb/run_db.go` to satisfy repo lint gates without changing store behavior.
- Adjusted plan tests to avoid shared-home run id collisions during parallel execution.

## Ready for Next Run
- Transport and daemon snapshot tasks can build on `internal/store/rundb` projection/query helpers and the home-scoped `run.db` allocation seam.
- Reconciliation and later cleanup tasks can continue using the compatibility `events.jsonl` mirror until every legacy reader is migrated off it.
